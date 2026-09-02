//go:build native

// The desktop dev client: the same game as the wasm build, opening on
// the same start screen the web serves — type a name, Enter joins. The
// production server is baked in; -server overrides it and -name skips
// the start screen. Built by `make native` with -tags native (the one
// target that needs cgo).
package main

import (
	"errors"
	"flag"
	"log"

	"github.com/hajimehoshi/ebiten/v2"

	"flappy-race/internal/client"
	"flappy-race/internal/protocol"
)

// defaultServer is the instance this binary plays against out of the box.
const defaultServer = "wss://birds.klimlive.de/ws"

func main() {
	server := flag.String("server", defaultServer, "game server WebSocket URL")
	name := flag.String("name", "", "player name (empty: pick one on the start screen)")
	flag.Parse()

	ebiten.SetWindowTitle("Flappy Race")
	ebiten.SetWindowResizable(true)
	ebiten.SetWindowSize(protocol.CanvasW, protocol.CanvasH)
	g := client.NewGame(*server, *name)
	if err := ebiten.RunGame(g); err != nil && !errors.Is(err, ebiten.Termination) {
		log.Printf("game exited: %v", err)
	}
}