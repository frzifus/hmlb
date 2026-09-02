// The flappy-race server: serves the embedded web shell and the wasm game
// client, and hosts the authoritative race simulation behind /ws.
package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"flappy-race/internal/server"
	"flappy-race/internal/web"
)

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	data := flag.String("data", "data/state.json", "leaderboard state file")
	flag.Parse()

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))

	store, err := server.OpenStore(*data)
	if err != nil {
		slog.Warn("leaderboard state unusable, starting fresh", "err", err)
	}

	hub := server.NewHub(server.DefaultConfig(), store)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go hub.Run(ctx)

	srv := &http.Server{
		Addr:              *addr,
		Handler:           server.Handler(hub, store, web.FS()),
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		slog.Info("flappy race serving", "addr", *addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("listen failed", "err", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	slog.Info("shutting down")
	cancel()
	shutdownCtx, c := context.WithTimeout(context.Background(), 5*time.Second)
	defer c()
	_ = srv.Shutdown(shutdownCtx)
}