// Package web embeds the static shell of the game. index.html is
// committed; game.wasm and wasm_exec.js are produced by `make assets` and
// gitignored — the embed pattern compiles on a fresh clone because
// index.html always exists, and a missing wasm is handled with a helpful
// message by the server.
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:static
var staticFS embed.FS

// FS returns the static assets rooted for http.FileServerFS.
func FS() fs.FS {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic(err) // embed layout is fixed at compile time
	}
	return sub
}