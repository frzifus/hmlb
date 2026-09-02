//go:build js && wasm

// The wasm entry point: derives the WebSocket URL from the page location
// and the username from the start screen's JS global, then runs the game.
package main

import (
	"errors"
	"log"
	"syscall/js"

	"github.com/hajimehoshi/ebiten/v2"

	"flappy-race/internal/client"
)

func main() {
	loc := js.Global().Get("document").Get("location")
	scheme := "ws://"
	if loc.Get("protocol").String() == "https:" {
		scheme = "wss://"
	}
	url := scheme + loc.Get("host").String() + "/ws"

	name := ""
	nameVal := js.Global().Get("__flappyName")
	if !nameVal.IsUndefined() {
		name = nameVal.String()
	}

	ebiten.SetWindowTitle("Flappy Race")
	g := client.NewGame(url, name)
	if err := ebiten.RunGame(g); err != nil && !errors.Is(err, ebiten.Termination) {
		log.Printf("game exited: %v", err)
	}
}