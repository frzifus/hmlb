//go:build js && wasm || native

package draw

import (
	"fmt"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"flappy-race/internal/protocol"
)

// HUDState is the flat data the HUD renderer needs. The client derives it
// from the authoritative snapshot every frame (see client/hud.go) and
// converts; the draw package stays independent of the client package.
type HUDState struct {
	St        uint8
	Countdown string
	Go        bool
	OwnScore  int
	Wait      int
	Dead      bool
	Spect     bool
	Rows      []HUDRow
	Results   []ResultRow
}

type HUDRow struct {
	Rank              int
	Name              string
	Score             int
	Alive, Left, Self bool
}

type ResultRow struct {
	Rank  int
	Name  string
	Score int
	Left  bool
}

var (
	panelBG = color.RGBA{10, 14, 24, 110}
	textHi  = color.RGBA{255, 255, 255, 255}
	textDim = color.RGBA{255, 255, 255, 130}
	accGold = color.RGBA{255, 217, 51, 255}
)

// DrawHUD renders the overlay: countdown, own score, live leaderboard,
// banners and the results podium. viewW is the logical view width, which
// adapts to the screen; vertical layout stays on the fixed world height.
func DrawHUD(dst *ebiten.Image, txt *Texter, sc *Scenery, h HUDState, viewW float64) {
	hudClr := sc.HUDColor()

	// Own score, top center.
	txt.DrawShadowed(dst, fmt.Sprintf("%d", h.OwnScore), 44, viewW/2, 14, hudClr, true)

	// Countdown / GO flash, big and centered.
	switch {
	case h.Go:
		txt.DrawShadowed(dst, "LET'S GO!", 46, viewW/2, 300, accGold, true)
	case h.Countdown != "":
		txt.DrawShadowed(dst, h.Countdown, 110, viewW/2, 260, hudClr, true)
	}

	// Banners under the score.
	y := 64.0
	switch {
	case h.Spect && h.St == protocol.StRacing:
		txt.DrawShadowed(dst, "Race in progress — you fly in the next one!", 17, viewW/2, y, hudClr, true)
		y += 24
		if h.Wait > 1 {
			txt.DrawShadowed(dst, fmt.Sprintf("%d waiting for the next race", h.Wait), 14, viewW/2, y, hudClr, true)
			y += 20
		}
	case h.Dead:
		txt.DrawShadowed(dst, "You're out — spectating!", 17, viewW/2, y, hudClr, true)
		y += 24
	}

	// Live leaderboard, top right.
	drawLeaderboard(dst, txt, h.Rows, viewW-196, y+10)

	// Results podium replaces everything in the finished state.
	if h.St == protocol.StFinished {
		drawPodium(dst, txt, h.Results, viewW)
	}
}

func drawLeaderboard(dst *ebiten.Image, txt *Texter, rows []HUDRow, x, y float64) {
	const rowH = 20
	w := 186.0
	hgt := 14 + rowH*len(rows)
	vector.FillRect(dst, float32(x), float32(y), float32(w), float32(hgt), panelBG, true)
	txt.Draw(dst, "Leaderboard", 13, x+w/2, y+4, textDim, true)
	for i, r := range rows {
		clr := textDim
		if r.Alive {
			clr = textDim
			if r.Self {
				clr = accGold
			}
		}
		name := r.Name
		if r.Left {
			name += " (left)"
		}
		txt.DrawShadowed(dst, fmt.Sprintf("%d. %s", r.Rank, name), 13, x+10, y+22+float64(i)*rowH, clr, false)
		txt.DrawShadowed(dst, fmt.Sprintf("%d", r.Score), 13, x+w-14, y+22+float64(i)*rowH, clr, false)
	}
}

func drawPodium(dst *ebiten.Image, txt *Texter, res []ResultRow, viewW float64) {
	if len(res) == 0 {
		return
	}
	const rowH = 22
	h := 74 + rowH*len(res)
	x := viewW/2 - 130
	y := float64(protocol.CanvasH)*0.32 - float64(h)/2
	vector.FillRect(dst, float32(x), float32(y), 260, float32(h), panelBG, true)
	vector.FillRect(dst, float32(x), float32(y), 260, 2, accGold, true)
	vector.FillRect(dst, float32(x), float32(y+float64(h)-2), 260, 2, accGold, true)

	txt.DrawShadowed(dst, "Race results", 20, x+130, y+10, accGold, true)
	for i, r := range res {
		clr := textDim
		switch r.Rank {
		case 1:
			clr = accGold
		case 2:
			clr = color.RGBA{210, 210, 210, 255}
		case 3:
			clr = color.RGBA{205, 130, 80, 255}
		}
		name := r.Name
		if r.Left {
			name += " (left)"
		}
		txt.DrawShadowed(dst, fmt.Sprintf("%d. %s", r.Rank, name), 16, x+18, y+42+float64(i)*rowH, clr, false)
		txt.DrawShadowed(dst, fmt.Sprintf("%d", r.Score), 16, x+242, y+42+float64(i)*rowH, clr, false)
	}
	txt.Draw(dst, "next race starts in a moment…", 13, x+130, y+float64(h)-16, textDim, true)
}