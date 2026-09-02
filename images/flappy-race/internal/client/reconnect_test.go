//go:build native

// The reconnect e2e, headless: it drives Game.Update — the same loop the
// browser runs, minus rendering — against a real in-process server, stops
// that server, starts a fresh one on the same port (what a restart looks
// like from outside) and proves the client re-dials, re-joins and renders
// the new epoch's snapshots. Needs the `native` tag (and thus cgo, like
// `make native`); `make test-native` runs it.
package client

import (
	"context"
	"net"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"flappy-race/internal/protocol"
	"flappy-race/internal/server"
)

// epoch is one server lifetime. Successive epochs reuse the port and the
// state file, so the second epoch is exactly what the first one looks
// like after a restart.
type epoch struct {
	wsURL  string
	srv    *httptest.Server
	cancel context.CancelFunc
}

func startEpoch(t *testing.T, addr, dataPath string) *epoch {
	t.Helper()
	cfg := server.Config{
		DataPath:  dataPath,
		Countdown: 300 * time.Millisecond,
		Results:   800 * time.Millisecond,
	}
	store, err := server.OpenStore(dataPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	hub := server.NewHub(cfg, store)
	webfs := fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("<!doctype html>stub>")}}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatalf("listen %s: %v", addr, err)
	}
	srv := httptest.NewUnstartedServer(server.Handler(hub, store, webfs))
	srv.Listener = ln
	srv.Start()
	ctx, cancel := context.WithCancel(context.Background())
	go hub.Run(ctx)
	return &epoch{
		wsURL:  "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws",
		srv:    srv,
		cancel: cancel,
	}
}

func (e *epoch) stop() {
	e.cancel()
	e.srv.CloseClientConnections()
	e.srv.Close()
}

func TestGameReconnectsAcrossServerRestart(t *testing.T) {
	dataPath := t.TempDir() + "/state.json"

	e1 := startEpoch(t, "127.0.0.1:0", dataPath)
	defer e1.stop() // idempotent; the explicit stop below ends epoch 1
	addr := e1.srv.Listener.Addr().String()

	g := NewGame(e1.wsURL, "recon")

	// drive pumps Update at ~60 TPS, the way the game loop would.
	drive := func(timeout time.Duration, what string, cond func() bool) {
		t.Helper()
		deadline := time.Now().Add(timeout)
		for time.Now().Before(deadline) {
			if cond() {
				return
			}
			_ = g.Update()
			time.Sleep(16 * time.Millisecond)
		}
		t.Fatalf("timeout waiting for %s", what)
	}

	// Epoch 1: dial, join, receive snapshots; let one race finish so the
	// player's bird and stats hit disk before the restart.
	drive(5*time.Second, "first snapshots", func() bool { _, ok := g.ring.Newest(); return ok })
	_, pal1, acc1 := DefaultSess.Get()
	if name := DefaultSess.Name(); name != "recon" {
		t.Fatalf("welcomed as %q, want recon", name)
	}
	drive(5*time.Second, "a finished race", func() bool {
		s, ok := g.ring.Newest()
		return ok && s.St == protocol.StFinished
	})
	e1.stop()

	// The loss must move the game into its retry state, never kill it.
	drive(5*time.Second, "connection loss detection", func() bool { return g.net == nil })
	if g.fatal != "" {
		t.Fatalf("connection loss must not be terminal: %q", g.fatal)
	}

	// Epoch 2: a fresh hub on the same port — the server "restarted".
	e2 := startEpoch(t, addr, dataPath)
	defer e2.stop()
	drive(10*time.Second, "reconnect", func() bool { return g.net != nil })
	if g.attempts != 0 {
		t.Fatalf("attempts must reset after a successful join, got %d", g.attempts)
	}

	// The rejoin must yield epoch-2 snapshots listing the same player
	// (same bird: identity persisted per username across the restart).
	var last protocol.Snapshot
	drive(5*time.Second, "epoch-2 snapshots", func() bool {
		s, ok := g.ring.Newest()
		if !ok {
			return false
		}
		last = s
		return true
	})
	if _, pal2, acc2 := DefaultSess.Get(); pal2 != pal1 || acc2 != acc1 {
		t.Fatalf("bird changed across restart: %d/%d → %d/%d", pal1, acc1, pal2, acc2)
	}
	var found bool
	for _, b := range last.Birds {
		if b.Name == "recon" {
			found = true
		}
	}
	if !found {
		t.Fatalf("epoch-2 snapshot must list the rejoined player: %+v", last.Birds)
	}
}