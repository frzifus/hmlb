// Package racetest exercises the whole server stack headlessly: a real HTTP
// server, real WebSocket connections and the live 60 Hz hub, driven by
// scripted clients. The game itself never needs a GUI to be verified.
package racetest

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/gorilla/websocket"

	"flappy-race/internal/protocol"
	"flappy-race/internal/server"
)

// ————— harness —————

type gameServer struct {
	t        *testing.T
	httpURL  string
	wsURL    string
	dataPath string
	cancel   context.CancelFunc
	srv      *httptest.Server
}

func startServer(t *testing.T, mutate func(*server.Config)) *gameServer {
	t.Helper()
	cfg := server.Config{
		DataPath:  t.TempDir() + "/state.json",
		Countdown: 300 * time.Millisecond,
		Results:   800 * time.Millisecond,
	}
	if mutate != nil {
		mutate(&cfg)
	}
	store, err := server.OpenStore(cfg.DataPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	hub := server.NewHub(cfg, store)
	webfs := fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("<!doctype html>stub")}}
	srv := httptest.NewServer(server.Handler(hub, store, webfs))
	ctx, cancel := context.WithCancel(context.Background())
	go hub.Run(ctx)
	t.Cleanup(func() {
		cancel()
		srv.CloseClientConnections()
		srv.Close()
	})
	return &gameServer{
		t:        t,
		httpURL:  srv.URL,
		wsURL:    "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws",
		dataPath: cfg.DataPath,
		cancel:   cancel,
		srv:      srv,
	}
}

// testClient is one scripted player with a background reader.
type testClient struct {
	t    *testing.T
	g    *gameServer
	ws   *websocket.Conn
	id   uint64
	name string
	bird protocol.Bird

	mu        sync.Mutex
	first     protocol.Snapshot // first snapshot seen (pre-GO state)
	gotFirst  bool
	last      protocol.Snapshot
	snaps     int
	spect     bool
	readErr   chan error // terminal read error, closed-loop signaled once

	stopPilot chan struct{}
}

func dial(t *testing.T, g *gameServer, name string) *testClient {
	t.Helper()
	ws, _, err := websocket.DefaultDialer.Dial(g.wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	c := &testClient{t: t, g: g, ws: ws, name: name, readErr: make(chan error, 1)}
	c.sendJSON(protocol.JoinMsg{T: protocol.TJoin, Name: name})
	ws.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, data, err := ws.ReadMessage()
	if err != nil {
		t.Fatalf("join read: %v", err)
	}
	var w protocol.WelcomeMsg
	if json.Unmarshal(data, &w) != nil || w.T != protocol.TWelcome {
		t.Fatalf("expected welcome, got: %s", data)
	}
	c.bird = w.Bird
	c.spect = w.Spect
	c.id = w.You
	if !w.Bird.Valid() {
		t.Fatalf("welcome bird invalid: %+v", w.Bird)
	}
	ws.SetReadDeadline(time.Time{}) // back to no deadline for the reader
	go c.readLoop()
	return c
}

func (c *testClient) readLoop() {
	for {
		_, data, err := c.ws.ReadMessage()
		if err != nil {
			select {
			case c.readErr <- err:
			default:
			}
			return
		}
		var s protocol.Snapshot
		if json.Unmarshal(data, &s) != nil || s.T != protocol.TSnapshot {
			continue
		}
		c.mu.Lock()
		if !c.gotFirst {
			c.first = s
			c.gotFirst = true
		}
		c.last = s
		c.snaps++
		c.mu.Unlock()
	}
}

func (c *testClient) firstSeen() (protocol.Snapshot, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.first, c.gotFirst
}

func (c *testClient) sendJSON(v any) {
	c.t.Helper()
	b, _ := json.Marshal(v)
	_ = c.ws.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if err := c.ws.WriteMessage(websocket.TextMessage, b); err != nil {
		c.t.Fatalf("send: %v", err)
	}
}

func (c *testClient) flap()      { c.sendJSON(protocol.FlapMsg{T: protocol.TFlap}) }
func (c *testClient) closeConn() {
	if c.stopPilot != nil {
		close(c.stopPilot)
		c.stopPilot = nil
	}
	_ = c.ws.Close()
}

func (c *testClient) latest() (protocol.Snapshot, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.last, c.last.T == protocol.TSnapshot
}

// latestMust returns the newest snapshot or fails the test.
func (c *testClient) latestMust() protocol.Snapshot {
	c.t.Helper()
	s, ok := c.latest()
	if !ok {
		c.t.Fatal("no snapshot received")
	}
	return s
}

func (c *testClient) snapCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.snaps
}

// waitFor polls cond until it holds or the timeout expires.
func waitFor(t *testing.T, timeout time.Duration, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for %s", what)
}

// waitForState blocks until the client has seen a snapshot in state st.
func (c *testClient) waitForState(st uint8) protocol.Snapshot {
	c.t.Helper()
	var out protocol.Snapshot
	waitFor(c.t, 10*time.Second, fmt.Sprintf("state %d", st), func() bool {
		s, ok := c.latest()
		if ok && s.St == st {
			out = s
			return true
		}
		return false
	})
	return out
}

// birdOf finds a bird entry by id in a snapshot.
func birdOf(s protocol.Snapshot, id uint64) (protocol.BirdSnap, bool) {
	for _, b := range s.Birds {
		if b.ID == id {
			return b, true
		}
	}
	return protocol.BirdSnap{}, false
}

// autoPilot is a simple AI that hovers its bird around the next gap center —
// just good enough to outlive idle players while the test runs.
func (c *testClient) autoPilot() {
	c.stopPilot = make(chan struct{})
	stop := c.stopPilot
	go func() {
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				s, ok := c.latest()
				if !ok || s.St != protocol.StRacing {
					continue
				}
				me, ok := birdOf(s, c.id)
				if !ok || me.Dead {
					continue
				}
				target := protocol.SpawnY
				for _, p := range s.Pipes {
					if p.X+protocol.PipeWidth > protocol.BirdX-protocol.BirdRadius {
						target = p.G
						break
					}
				}
				if me.Y > target+6 {
					c.flap()
				}
			}
		}
	}()
}

func getJSON(t *testing.T, url string, into any) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s: status %d", url, resp.StatusCode)
	}
	if err := json.Unmarshal(body, into); err != nil {
		t.Fatalf("GET %s: bad json: %v (%s)", url, err, body)
	}
}

// ————— the lifecycle test —————

func TestFullRaceLifecycle(t *testing.T) {
	g := startServer(t, nil)

	// Static shell, missing-wasm explanation, health.
	resp, err := http.Get(g.httpURL + "/")
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("index: %v %v", err, resp)
	}
	resp.Body.Close()
	resp, err = http.Get(g.httpURL + "/game.wasm")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound || !strings.Contains(string(body), "make assets") {
		t.Fatalf("missing wasm must be explained, got %d: %s", resp.StatusCode, body)
	}
	resp, err = http.Get(g.httpURL + "/healthz")
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("healthz: %v %v", err, resp)
	}
	resp.Body.Close()

	// — Phase 1: three players join back-to-back; the first one starts a race. —
	ana := dial(t, g, "ana")
	waitFor(t, 5*time.Second, "countdown start", func() bool {
		s, ok := ana.latest()
		return ok && s.St == protocol.StCountdown
	})
	bob := dial(t, g, "bob")
	cid := dial(t, g, "cid")

	if ana.spect || bob.spect || cid.spect {
		t.Fatal("pre-GO joins must race, not spectate")
	}
	if ana.id == bob.id || bob.id == cid.id || ana.id == cid.id {
		t.Fatalf("ids must be unique: %d %d %d", ana.id, bob.id, cid.id)
	}

	// — Phase 2: GO! ana and cid never flap; bob flies with the autopilot. —
	s1 := ana.waitForState(protocol.StRacing)
	race1Theme := s1.Theme
	if d := s1.GoAt - s1.Now; d > 300 {
		t.Fatalf("countdown window %dms > 300", d)
	}
	anaBird1, _ := birdOf(s1, ana.id)

	bob.autoPilot()

	// ana and cid idle: both must die within ~1 s of GO.
	waitFor(t, 5*time.Second, "idle players dying", func() bool {
		s, ok := bob.latest()
		if !ok || s.St != protocol.StRacing {
			return false
		}
		ba, _ := birdOf(s, ana.id)
		bc, _ := birdOf(s, cid.id)
		return ba.Dead && bc.Dead
	})

	// Dead players stay visible with frozen state (requirement 5).
	snap := bob.latestMust()
	for _, id := range []uint64{ana.id, cid.id} {
		b, ok := birdOf(snap, id)
		if !ok || !b.Dead {
			t.Fatalf("dead bird %d must stay in the snapshot: %+v", id, snap.Birds)
		}
	}

	// The autopilot scores, then disconnects — the last alive racer leaves,
	// which must end the race with bob ranked first.
	waitFor(t, 10*time.Second, "bob scoring", func() bool {
		s, ok := bob.latest()
		if !ok {
			return false
		}
		b, ok := birdOf(s, bob.id)
		return ok && b.Score >= 1 && !b.Dead
	})
	bobBird := bob.bird
	bob.closeConn() // stops the pilot implicitly

	// — Phase 3: results window. —
	res := ana.waitForState(protocol.StFinished)
	if len(res.Res) != 3 {
		t.Fatalf("results must list everyone, got %+v", res.Res)
	}
	if res.Res[0].Name != "bob" || res.Res[0].Score <= 0 {
		t.Fatalf("bob (survivor) must win: %+v", res.Res)
	}
	if res.Res[1].Name != "ana" || res.Res[2].Name != "cid" {
		t.Fatalf("idle ties must break by id asc: %+v", res.Res)
	}

	// — Phase 4: persistence — leaderboard API, state file, stored bird. —
	var lb struct {
		Top []protocol.TopEntry `json:"top"`
	}
	getJSON(t, g.httpURL+"/api/leaderboard", &lb)
	if len(lb.Top) == 0 || lb.Top[0].Name != "bob" {
		t.Fatalf("all-time top wrong: %+v", lb.Top)
	}
	if _, err := os.Stat(g.dataPath); err != nil {
		t.Fatalf("state file must exist: %v", err)
	}
	bob2 := dial(t, g, "bob")
	if bob2.bird != bobBird {
		t.Fatalf("reconnecting bob must keep its bird: %+v vs %+v", bob2.bird, bobBird)
	}

	// — Phase 5: late joiner during the results window → races next. —
	dave := dial(t, g, "dave")
	if !dave.spect {
		t.Fatal("join during the results window must be a spectator")
	}

	// — Phase 6: race 2 — new theme, session birds persist, dave races. —
	s2 := dave.waitForState(protocol.StCountdown)
	if s2.Theme == race1Theme {
		t.Fatalf("theme repeated between races: %d", s2.Theme)
	}
	s2 = dave.waitForState(protocol.StRacing)
	if _, ok := birdOf(s2, dave.id); !ok {
		t.Fatalf("spectator must become a racer: %+v", s2.Birds)
	}
	if _, ok := birdOf(s2, bob2.id); !ok {
		t.Fatalf("returning player must race: %+v", s2.Birds)
	}
	if ba, _ := birdOf(s2, ana.id); ba.Pal != anaBird1.Pal || ba.Acc != anaBird1.Acc {
		t.Fatalf("session bird must persist across races: %+v vs %+v", ba, anaBird1)
	}

	// — Phase 7: snapshot cadence ≈30 Hz. —
	before := ana.snapCount()
	time.Sleep(1 * time.Second)
	if rate := ana.snapCount() - before; rate < 25 {
		t.Fatalf("snapshot rate %d/s < 25", rate)
	}

	// — Phase 8: disconnect storm — hub returns to idle; a fresh joiner
	// starts its own countdown. —
	ana.closeConn()
	cid.closeConn()
	bob2.closeConn()
	dave.closeConn()
	eve := dial(t, g, "eve")
	waitFor(t, 5*time.Second, "eve's first snapshot", func() bool {
		_, ok := eve.firstSeen()
		return ok
	})
	efs, _ := eve.firstSeen()
	if efs.St != protocol.StCountdown {
		t.Fatalf("fresh join after total disconnect must start a countdown, got %d", efs.St)
	}

	// — Phase 9: hostile clients — garbage tolerated, oversize dropped. —
	trash := dial(t, g, "trash")
	_ = trash.ws.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if err := trash.ws.WriteMessage(websocket.TextMessage, []byte(`{"t":99,"junk`)); err != nil {
		t.Fatalf("garbage write: %v", err)
	}
	trash.sendJSON(map[string]any{"t": 99, "x": []string{"a"}})
	// The oversize frame must kill only trash's connection — the terminal
	// read error is reported by trash's own reader goroutine…
	_ = trash.ws.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if err := trash.ws.WriteMessage(websocket.TextMessage, make([]byte, 8192)); err != nil {
		t.Fatalf("oversize write: %v", err)
	}
	select {
	case err := <-trash.readErr:
		if err == nil {
			t.Fatal("expected a read error for the dropped client")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("oversize client must be dropped")
	}
	// …while eve keeps receiving snapshots.
	before = eve.snapCount()
	time.Sleep(400 * time.Millisecond)
	if eve.snapCount() <= before {
		t.Fatal("healthy client must keep receiving snapshots after hostile input")
	}
}