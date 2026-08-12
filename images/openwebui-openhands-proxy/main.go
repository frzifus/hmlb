// openwebui-openhands-proxy sits between OpenWebUI and the OpenHands
// agent-server's OpenAI-compatible gateway (/v1/chat/completions).
//
// Problem it solves: the OpenHands gateway keeps multi-turn agent state under a
// server-generated conversation ID returned in the X-OpenHands-ServerConversation-ID
// RESPONSE header. To continue a conversation, the client must echo that ID as a
// REQUEST header on follow-up turns. OpenWebUI (and its Android app) can send
// static custom request headers but cannot capture a response header and replay
// it — so without this proxy every message starts a fresh OpenHands conversation
// and the agent loses its workspace/sandbox state.
//
// What it does: for each /v1/chat/completions request it derives a stable thread
// key from the first user message (OpenWebUI sends the full history every turn,
// so the first user message is invariant for a chat thread), looks up a stored
// OpenHands conversation ID for that key, injects X-OpenHands-ServerConversation-ID
// on the upstream request if present, then captures the ID from the upstream
// response and remembers it for the key. Stale IDs (upstream 4xx/5xx after an
// injection) self-heal with a single retry without the header.
//
// The key→conversation-ID mapping is persisted to a JSON file (CONV_STORE_FILE)
// so it survives proxy restarts. Entries are kept forever (no TTL).
//
// Everything else (e.g. GET /v1/models) is reverse-proxied transparently, with
// the caller's Authorization header passed through unchanged.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const convHeader = "X-OpenHands-ServerConversation-ID"

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

var (
	upstreamURL = env("OPENHANDS_UPSTREAM_URL", "http://openhands-agent-server.llm.svc.cluster.local:18000")
	port        = env("PORT", env("PROXY_PORT", "8080"))
	storeFile   = env("CONV_STORE_FILE", "/data/conversations.json")
)

// --- persistent conversation-ID store --------------------------------------

type store struct {
	mu      sync.Mutex
	entries map[string]string // thread key -> OpenHands conversation ID
	path    string
}

func newStore(path string) *store {
	s := &store{entries: make(map[string]string), path: path}
	s.load()
	return s
}

// load reads the persisted mapping if present; a missing/corrupt file is
// non-fatal — we just start with an empty map.
func (s *store) load() {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, err := os.ReadFile(s.path)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("store: read %s failed, starting empty: %v", s.path, err)
		}
		return
	}
	if err := json.Unmarshal(b, &s.entries); err != nil {
		log.Printf("store: parse %s failed, starting empty: %v", s.path, err)
		s.entries = make(map[string]string)
	}
	log.Printf("store: loaded %d conversation mapping(s) from %s", len(s.entries), s.path)
}

// save writes the mapping atomically (temp file + rename) so a crash mid-write
// can't leave a truncated file. Caller must hold s.mu.
func (s *store) save() {
	if s.path == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		log.Printf("store: mkdir %s failed: %v", filepath.Dir(s.path), err)
		return
	}
	b, err := json.MarshalIndent(s.entries, "", "  ")
	if err != nil {
		log.Printf("store: marshal failed: %v", err)
		return
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		log.Printf("store: write %s failed: %v", tmp, err)
		return
	}
	if err := os.Rename(tmp, s.path); err != nil {
		log.Printf("store: rename %s -> %s failed: %v", tmp, s.path, err)
		return
	}
}

func (s *store) get(key string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id, ok := s.entries[key]
	return id, ok
}

func (s *store) set(key, id string) {
	if key == "" || id == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.entries[key] == id {
		return // no change, skip the write
	}
	s.entries[key] = id
	s.save()
}

// --- request parsing -------------------------------------------------------

// extractThreadKey returns a stable hash of the first user message content.
// OpenWebUI sends the full message history on every request, so the first user
// message is constant across all turns of one chat thread. Two distinct chats
// that happen to start with the identical first user message would collide and
// share an OpenHands conversation — an acceptable, documented trade-off until
// OpenWebUI can send a per-chat identifier we can key on instead.
func extractThreadKey(body []byte) string {
	var req struct {
		Messages []struct {
			Role    string      `json:"role"`
			Content interface{} `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		return ""
	}
	for _, m := range req.Messages {
		if strings.EqualFold(m.Role, "user") {
			return hashContent(m.Content)
		}
	}
	return ""
}

// hashContent handles both plain-string content and the array-of-parts form
// used for multimodal messages (it concatenates the text parts).
func hashContent(c interface{}) string {
	var text string
	switch v := c.(type) {
	case string:
		text = v
	case []interface{}:
		var b strings.Builder
		for _, p := range v {
			if pm, ok := p.(map[string]interface{}); ok {
				if t, ok := pm["text"].(string); ok {
					b.WriteString(t)
				}
			}
		}
		text = b.String()
	default:
		// Fall back to a JSON rendering so we still produce a stable key.
		if b, err := json.Marshal(c); err == nil {
			text = string(b)
		}
	}
	h := sha256.Sum256([]byte(text))
	return hex.EncodeToString(h[:])
}

// --- chat completions handler ----------------------------------------------

func chatCompletionsHandler(s *store, upstream *url.URL, timeout time.Duration) http.HandlerFunc {
	// Agent task loops can run long (multiple LLM calls); keep a generous
	// client-side ceiling so we don't cut off a working agent mid-task.
	client := &http.Client{Timeout: timeout}
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "read request body: "+err.Error(), http.StatusBadRequest)
			return
		}
		_ = r.Body.Close()

		key := extractThreadKey(body)
		convID, hadMapping := "", false
		if key != "" {
			convID, hadMapping = s.get(key)
		}

		// doForward sends the captured body upstream, optionally injecting the
		// conversation-ID header. The caller's Authorization header is preserved.
		doForward := func(injectID string) (*http.Response, error) {
			target := upstream.String() + r.URL.Path
			if r.URL.RawQuery != "" {
				target += "?" + r.URL.RawQuery
			}
			upReq, err := http.NewRequestWithContext(r.Context(), r.Method, target, bytes.NewReader(body))
			if err != nil {
				return nil, err
			}
			upReq.Header = r.Header.Clone()
			upReq.Host = upstream.Host
			upReq.ContentLength = int64(len(body))
			if injectID != "" {
				upReq.Header.Set(convHeader, injectID)
			} else {
				upReq.Header.Del(convHeader)
			}
			return client.Do(upReq)
		}

		resp, err := doForward(convID)
		if err != nil {
			http.Error(w, "upstream: "+err.Error(), http.StatusBadGateway)
			return
		}

		respConvID := resp.Header.Get(convHeader)

		// Self-healing: if we injected a (possibly stale) stored ID and the
		// upstream rejected it, retry once without the header to start a fresh
		// OpenHands conversation. Only retries when we actually injected — a
		// brand-new conversation that errors should surface as-is.
		if hadMapping && resp.StatusCode >= 400 {
			resp.Body.Close()
			if resp2, err2 := doForward(""); err2 == nil {
				resp = resp2
				respConvID = resp2.Header.Get(convHeader)
			}
		}

		// Remember/refresh the mapping for this thread.
		if key != "" && respConvID != "" {
			s.set(key, respConvID)
		}

		// Copy upstream headers through to the caller, and also surface the
		// conversation ID (harmless; handy for debugging even if OpenWebUI
		// ignores response headers).
		for k, vs := range resp.Header {
			if strings.EqualFold(k, convHeader) {
				continue
			}
			for _, v := range vs {
				w.Header().Add(k, v)
			}
		}
		if respConvID != "" {
			w.Header().Set(convHeader, respConvID)
		}
		w.WriteHeader(resp.StatusCode)
		_, _ = io.Copy(w, resp.Body)
		_ = resp.Body.Close()

		log.Printf("chat_completions key=%s inject=%v conv_id=%s status=%d",
			short(key), hadMapping, short(respConvID), resp.StatusCode)
	}
}

func short(s string) string {
	if len(s) <= 12 {
		return s
	}
	return s[:12]
}

// --- main ------------------------------------------------------------------

func main() {
	upstream, err := url.Parse(upstreamURL)
	if err != nil {
		log.Fatalf("bad OPENHANDS_UPSTREAM_URL %q: %v", upstreamURL, err)
	}
	s := newStore(storeFile)

	// Transparent reverse proxy for everything except chat completions
	// (GET /v1/models, /alive, /docs, ...). Preserves Authorization.
	proxy := httputil.NewSingleHostReverseProxy(upstream)
	proxy.Director = func(req *http.Request) {
		req.URL.Scheme = upstream.Scheme
		req.URL.Host = upstream.Host
		req.Host = upstream.Host
		req.Header.Del(convHeader) // never let a caller forge continuity
	}

	health := func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "ok")
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/chat/completions", chatCompletionsHandler(s, upstream, 30*time.Minute))
	mux.HandleFunc("/healthz", health)
	mux.HandleFunc("/alive", health)
	mux.HandleFunc("/", proxy.ServeHTTP)

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Printf("openwebui-openhands-proxy listening on :%s upstream=%s store=%s", port, upstreamURL, storeFile)
	log.Fatal(srv.ListenAndServe())
}