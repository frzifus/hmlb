package server

import (
	"fmt"
	"io/fs"
	"net/http"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	// The game is public and cookie-less; the browser's WebSocket URL is
	// always same-origin anyway. Allow every origin (also helps when the
	// eventual reverse proxy rewrites Host).
	CheckOrigin: func(*http.Request) bool { return true },
}

// Handler wires the hub, the leaderboard API and the embedded web assets
// into one http.Handler.
func Handler(hub *Hub, store *Store, webfs fs.FS) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /ws", func(w http.ResponseWriter, r *http.Request) {
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return // upgrader already replied
		}
		hub.serveConn(newClientConn(ws))
	})

	mux.HandleFunc("GET /api/leaderboard", func(w http.ResponseWriter, r *http.Request) {
		b := store.APIBytes()
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(b)
	})

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "ok")
	})

	// Static assets: index.html (committed), game.wasm + wasm_exec.js
	// (built by `make assets`). Serve a helpful pointer when the wasm has
	// not been built yet instead of a bare 404.
	fileServer := http.FileServerFS(webfs)
	mux.HandleFunc("GET /game.wasm", func(w http.ResponseWriter, r *http.Request) {
		if _, err := fs.Stat(webfs, "game.wasm"); err != nil {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusNotFound)
			fmt.Fprintln(w, "game.wasm not built yet — run `make assets` (or `make build`) first.")
			return
		}
		fileServer.ServeHTTP(w, r)
	})
	mux.Handle("GET /", fileServer)

	return mux
}