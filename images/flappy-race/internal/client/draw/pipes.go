//go:build js && wasm || native

package draw

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"flappy-race/internal/protocol"
)

var (
	pipeW    = float32(protocol.PipeWidth)
	pipeLipH = float32(10)
	pipeHi   = color.RGBA{255, 255, 255, 60}
)

// DrawPipe renders one pipe pair (the columns above and below the gap) in
// the current theme's colors.
func (sc *Scenery) DrawPipe(dst *ebiten.Image, x, gapY, gapH float32) {
	topEnd := gapY - gapH/2
	botY := gapY + gapH/2

	sc.column(dst, x, 0, topEnd)
	sc.column(dst, x, botY, gtop())
	// Gap lips.
	vector.FillRect(dst, x-4, topEnd-pipeLipH, pipeW+8, pipeLipH, sc.pipeEdge, true)
	vector.FillRect(dst, x-4, botY, pipeW+8, pipeLipH, sc.pipeEdge, true)
}

// column fills a vertical pipe segment with body color, side shading and a
// highlight stripe.
func (sc *Scenery) column(dst *ebiten.Image, x, y0, y1 float32) {
	h := y1 - y0
	if h <= 0 {
		return
	}
	vector.FillRect(dst, x, y0, pipeW, h, sc.pipe, false)
	// Side shading.
	vector.FillRect(dst, x, y0, 3, h, sc.pipeEdge, false)
	vector.FillRect(dst, x+pipeW-3, y0, 3, h, sc.pipeEdge, false)
	// Highlight stripe.
	vector.FillRect(dst, x+6, y0, 4, h, pipeHi, false)
}