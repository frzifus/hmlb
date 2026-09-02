//go:build js && wasm || native

package draw

import (
	"image"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"flappy-race/internal/protocol"
)

// The scenery is fully procedural: a static sky gradient, two parallax
// layers (wrap-around tiles) and a ground strip, all laid out
// deterministically from the race seed so every client draws identical
// scenery for a race.
const (
	sceneryTileW = float32(960)
	groundH      = float32(80)
)

// gtop is the y where the ground strip begins in full-canvas layers.
func gtop() float32 { return float32(protocol.CanvasH) - groundH }

type rgba [4]float32 // 0..1

func (c rgba) Color() color.RGBA {
	return color.RGBA{
		R: uint8(clamp01(c[0]) * 255),
		G: uint8(clamp01(c[1]) * 255),
		B: uint8(clamp01(c[2]) * 255),
		A: uint8(clamp01(c[3]) * 255),
	}
}

func clamp01(v float32) float32 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

type themeDef struct {
	skyTop, skyBot    rgba
	far, mid          rgba
	ground, groundAlt rgba
	pipe, pipeEdge    rgba
	textOnSky         color.RGBA // readable HUD color against this sky
}

var themes = [protocol.ThemeCount]themeDef{
	// Desert: warm sky, pyramids on the horizon, dunes in front.
	0: {
		skyTop: rgba{1.00, 0.85, 0.56, 1}, skyBot: rgba{1.00, 0.64, 0.37, 1},
		far: rgba{0.79, 0.54, 0.29, 1}, mid: rgba{0.91, 0.63, 0.36, 1},
		ground: rgba{0.94, 0.76, 0.44, 1}, groundAlt: rgba{0.86, 0.66, 0.36, 1},
		pipe: rgba{0.72, 0.50, 0.30, 1}, pipeEdge: rgba{0.52, 0.34, 0.19, 1},
		textOnSky: color.RGBA{90, 50, 20, 255},
	},
	// Clouds: clear blue, layered cumulus, green meadow ground.
	1: {
		skyTop: rgba{0.42, 0.72, 0.95, 1}, skyBot: rgba{0.75, 0.90, 1.00, 1},
		far: rgba{1, 1, 1, 0.85}, mid: rgba{1, 1, 1, 1},
		ground: rgba{0.45, 0.75, 0.35, 1}, groundAlt: rgba{0.36, 0.63, 0.28, 1},
		pipe: rgba{0.90, 0.94, 0.97, 1}, pipeEdge: rgba{0.55, 0.65, 0.75, 1},
		textOnSky: color.RGBA{20, 40, 70, 255},
	},
	// Forest: soft light, tree lines and hills.
	2: {
		skyTop: rgba{0.55, 0.78, 0.92, 1}, skyBot: rgba{0.80, 0.92, 0.72, 1},
		far: rgba{0.29, 0.54, 0.35, 1}, mid: rgba{0.23, 0.48, 0.29, 1},
		ground: rgba{0.31, 0.48, 0.23, 1}, groundAlt: rgba{0.24, 0.40, 0.18, 1},
		pipe: rgba{0.54, 0.42, 0.27, 1}, pipeEdge: rgba{0.30, 0.48, 0.22, 1},
		textOnSky: color.RGBA{25, 45, 25, 255},
	},
	// Underwater: deep gradient, kelp and coral silhouettes.
	3: {
		skyTop: rgba{0.10, 0.35, 0.54, 1}, skyBot: rgba{0.04, 0.16, 0.29, 1},
		far: rgba{0.10, 0.40, 0.55, 0.55}, mid: rgba{0.09, 0.30, 0.42, 0.9},
		ground: rgba{0.16, 0.35, 0.42, 1}, groundAlt: rgba{0.12, 0.28, 0.34, 1},
		pipe: rgba{0.85, 0.47, 0.41, 1}, pipeEdge: rgba{0.60, 0.28, 0.24, 1},
		textOnSky: color.RGBA{220, 235, 245, 255},
	},
}

// Scenery holds the pre-rendered layers of one race's theme.
type Scenery struct {
	theme              protocol.Theme
	Sky                *ebiten.Image
	Far, Mid           *ebiten.Image // wrap-around parallax tiles
	Ground             *ebiten.Image
	pipe, pipeEdge, hud color.RGBA
}

// BuildScenery renders all layers for a theme/seed combination.
func BuildScenery(theme protocol.Theme, seed uint64) *Scenery {
	def := themes[theme%protocol.ThemeCount]
	rng := &mix{state: seed ^ uint64(theme)*0x9E3779B97F4A7C15}
	s := &Scenery{
		theme:    theme,
		Sky:      skyImage(def),
		Far:      emptyTile(),
		Mid:      emptyTile(),
		Ground:   groundTile(def),
		pipe:     def.pipe.Color(),
		pipeEdge: def.pipeEdge.Color(),
		hud:      def.textOnSky,
	}
	switch theme {
	case protocol.ThemeDesert:
		drawPyramids(s.Far, def.far.Color(), rng)
		drawDunes(s.Mid, def.mid.Color(), rng)
	case protocol.ThemeClouds:
		drawCloudBand(s.Far, def.far.Color(), rng, 8)
		drawCloudBand(s.Mid, def.mid.Color(), rng, 6)
	case protocol.ThemeForest:
		drawHills(s.Far, def.far.Color(), rng)
		drawTrees(s.Mid, def.mid.Color(), rng)
	case protocol.ThemeUnderwater:
		drawRays(s.Far, def.far.Color(), rng)
		drawKelp(s.Mid, def.mid.Color(), rng)
		drawCoral(s.Mid, def.pipe.Color(), rng)
	}
	return s
}

// ————— builders —————

func skyImage(def themeDef) *ebiten.Image {
	img := image.NewRGBA(image.Rect(0, 0, protocol.CanvasW, protocol.CanvasH))
	a, b := def.skyTop, def.skyBot
	for y := 0; y < protocol.CanvasH; y++ {
		t := float32(y) / float32(protocol.CanvasH)
		c := rgba{
			a[0] + (b[0]-a[0])*t,
			a[1] + (b[1]-a[1])*t,
			a[2] + (b[2]-a[2])*t,
			1,
		}.Color()
		row := img.Pix[y*img.Stride : y*img.Stride+img.Stride]
		for x := 0; x < protocol.CanvasW; x++ {
			row[x*4] = c.R
			row[x*4+1] = c.G
			row[x*4+2] = c.B
			row[x*4+3] = c.A
		}
	}
	return ebiten.NewImageFromImage(img)
}

func emptyTile() *ebiten.Image {
	return ebiten.NewImage(int(sceneryTileW), protocol.CanvasH)
}

func groundTile(def themeDef) *ebiten.Image {
	img := ebiten.NewImage(int(sceneryTileW), int(groundH))
	vector.FillRect(img, 0, 0, sceneryTileW, groundH, def.ground.Color(), false)
	rng := &mix{state: 1234567}
	alt := def.groundAlt.Color()
	for range 90 {
		x := float32(rng.Next() % uint64(sceneryTileW))
		y := float32(6+rng.Next()%uint64(groundH-12))
		w := float32(2 + rng.Next()%6)
		vector.FillRect(img, x, y, w, w/2, alt, false)
	}
	return img
}

// ————— theme decorations —————

// mound paints a smooth wide hump (pyramids, dunes, hills) as stacked rects.
func mound(dst *ebiten.Image, x, w, h float32, clr color.Color) {
	for y := float32(0); y < h; y += 4 {
		cw := w * (1 - y/h)
		vector.FillRect(dst, x+(w-cw)/2, gtop()-y-4, cw, 4, clr, false)
	}
}

func drawPyramids(dst *ebiten.Image, clr color.Color, rng *mix) {
	for i := range 3 {
		x := 80 + float32(i)*320
		mound(dst, x, 120+float32(i)*30, 150+float32(i)*40, clr)
	}
	_ = rng
}

func drawDunes(dst *ebiten.Image, clr color.Color, rng *mix) {
	for x := float32(0); x < sceneryTileW; x += 160 {
		mound(dst, x, 180+float32(rng.Next()%80), 40+float32(rng.Next()%50), clr)
	}
}

func drawCloudBand(dst *ebiten.Image, clr color.Color, rng *mix, n int) {
	cloud := func(cx, cy, r float32) {
		vector.FillCircle(dst, cx, cy, r, clr, true)
		vector.FillCircle(dst, cx-r*0.8, cy+r*0.25, r*0.7, clr, true)
		vector.FillCircle(dst, cx+r*0.8, cy+r*0.3, r*0.65, clr, true)
	}
	for range n {
		cx := float32(rng.Next() % uint64(sceneryTileW))
		cy := 80 + float32(rng.Next()%420)
		cloud(cx, cy, 26+float32(rng.Next()%30))
	}
}

func drawHills(dst *ebiten.Image, clr color.Color, rng *mix) {
	for x := float32(0); x < sceneryTileW; x += 140 {
		mound(dst, x, 200+float32(rng.Next()%60), 70+float32(rng.Next()%70), clr)
	}
}

func drawTrees(dst *ebiten.Image, clr color.Color, rng *mix) {
	trunk := color.RGBA{90, 62, 40, 255}
	for x := float32(30); x < sceneryTileW; x += 95 {
		h := 90 + float32(rng.Next()%70)
		vector.FillRect(dst, x-5, gtop()-h, 10, h, trunk, false)
		canopy := clr
		top := gtop() - h
		vector.FillCircle(dst, x, top, 34+float32(rng.Next()%14), canopy, true)
		vector.FillCircle(dst, x-22, top+18, 24, canopy, true)
		vector.FillCircle(dst, x+22, top+18, 24, canopy, true)
	}
}

func drawRays(dst *ebiten.Image, clr color.Color, rng *mix) {
	for range 6 {
		x := float32(rng.Next() % uint64(sceneryTileW))
		w := 40 + float32(rng.Next()%60)
		for y := float32(0); y < float32(protocol.CanvasH); y += 8 {
			vector.FillRect(dst, x+y*0.35, y, w, 8, clr, false)
		}
	}
}

func drawKelp(dst *ebiten.Image, clr color.Color, rng *mix) {
	for x := float32(40); x < sceneryTileW; x += 70 {
		h := 180 + float32(rng.Next()%260)
		lean := float32(rng.Next()%40) - 20
		w := 10 + float32(rng.Next()%8)
		for y := float32(0); y < h; y += 6 {
			off := (y / h) * lean
			vector.FillRect(dst, x+off, gtop()-y-6, w, 6, clr, false)
		}
	}
}

func drawCoral(dst *ebiten.Image, clr color.Color, rng *mix) {
	for range 12 {
		x := float32(rng.Next() % uint64(sceneryTileW))
		r := 10 + float32(rng.Next()%22)
		vector.FillCircle(dst, x, gtop()-r*0.5, r, clr, true)
	}
}

// ————— a tiny local splitmix64 for deterministic scenery —————

type mix struct{ state uint64 }

func (m *mix) Next() uint64 {
	m.state += 0x9E3779B97F4A7C15
	z := m.state
	z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
	z = (z ^ (z >> 27)) * 0x94D049BB133111EB
	return z ^ (z >> 31)
}

// PipeColors exposes the theme's pipe body/edge colors.
func (t *Scenery) PipeColors() (body, edge color.RGBA) { return t.pipe, t.pipeEdge }

// HUDColor is the readable text color against this theme's sky.
func (t *Scenery) HUDColor() color.RGBA { return t.hud }