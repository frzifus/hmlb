//go:build js && wasm || native

package draw

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"flappy-race/internal/protocol"
)

// Bird sprites: one vector design parameterized by palette (12 body colors)
// and accessory, pre-rendered per wing frame. The sprite is drawn larger
// than the 11 px collision radius on purpose — fair-feeling hitboxes are
// the ones that are smaller than the visuals.
const (
	spriteW    = float32(44)
	spriteH    = float32(34)
	wingFrames = 3
)

// Sprite geometry for callers that position sprites by their center.
const (
	SpriteHalfW = float64(spriteW) / 2
	SpriteHalfH = float64(spriteH) / 2
	// NameOffset is the gap between the bird's center and its name label.
	NameOffset = 28.0
)

var palettes = [protocol.PaletteCount][3]rgba{
	{rgba{0.91, 0.36, 0.29, 1}, rgba{0.78, 0.25, 0.20, 1}, rgba{1.00, 0.60, 0.15, 1}}, // red
	{rgba{0.29, 0.66, 0.91, 1}, rgba{0.20, 0.50, 0.76, 1}, rgba{1.00, 0.60, 0.15, 1}}, // blue
	{rgba{0.95, 0.79, 0.30, 1}, rgba{0.85, 0.65, 0.20, 1}, rgba{0.95, 0.50, 0.10, 1}}, // yellow
	{rgba{0.44, 0.81, 0.49, 1}, rgba{0.32, 0.66, 0.38, 1}, rgba{1.00, 0.60, 0.15, 1}}, // green
	{rgba{0.66, 0.44, 0.91, 1}, rgba{0.52, 0.32, 0.76, 1}, rgba{1.00, 0.60, 0.15, 1}}, // purple
	{rgba{0.95, 0.60, 0.29, 1}, rgba{0.82, 0.46, 0.18, 1}, rgba{1.00, 0.75, 0.10, 1}}, // orange
	{rgba{0.95, 0.44, 0.70, 1}, rgba{0.82, 0.30, 0.56, 1}, rgba{1.00, 0.60, 0.15, 1}}, // pink
	{rgba{0.31, 0.88, 0.82, 1}, rgba{0.20, 0.72, 0.66, 1}, rgba{1.00, 0.60, 0.15, 1}}, // cyan
	{rgba{0.66, 0.47, 0.31, 1}, rgba{0.52, 0.35, 0.22, 1}, rgba{1.00, 0.60, 0.15, 1}}, // brown
	{rgba{0.94, 0.94, 0.94, 1}, rgba{0.78, 0.80, 0.84, 1}, rgba{1.00, 0.60, 0.15, 1}}, // white
	{rgba{0.29, 0.35, 0.42, 1}, rgba{0.18, 0.22, 0.28, 1}, rgba{1.00, 0.60, 0.15, 1}}, // slate
	{rgba{0.23, 0.75, 0.69, 1}, rgba{0.15, 0.60, 0.55, 1}, rgba{1.00, 0.60, 0.15, 1}}, // teal
}

var (
	beakColor = rgba{1.00, 0.62, 0.12, 1}.Color()
	eyeDark   = color.RGBA{20, 20, 20, 255}
)

// Sprites lazily pre-renders bird sprites per (palette, accessory, frame).
type Sprites struct {
	m map[[3]uint8]*ebiten.Image
}

func NewSprites() *Sprites {
	return &Sprites{m: map[[3]uint8]*ebiten.Image{}}
}

func (s *Sprites) Get(pal, acc, frame uint8) *ebiten.Image {
	key := [3]uint8{pal, acc, frame}
	if img, ok := s.m[key]; ok {
		return img
	}
	img := birdSprite(pal, acc, frame)
	s.m[key] = img
	return img
}

func birdSprite(pal, acc, frame uint8) *ebiten.Image {
	p := palettes[pal%protocol.PaletteCount]
	body, wing := p[0].Color(), p[1].Color()
	img := ebiten.NewImage(int(spriteW), int(spriteH))
	cx := spriteW / 2
	cy := spriteH / 2

	// Body: a slightly wide ellipse via two overlapping circles.
	vector.FillCircle(img, cx-2, cy, 14, body, true)
	vector.FillCircle(img, cx+4, cy+2, 12, body, true)
	// Belly highlight.
	vector.FillCircle(img, cx+3, cy+6, 8, color.RGBA{255, 255, 255, 70}, true)

	// Wing: one of three poses.
	switch frame % wingFrames {
	case 0: // raised
		vector.FillCircle(img, cx-8, cy-9, 8, wing, true)
	case 1: // mid
		vector.FillCircle(img, cx-10, cy-2, 8, wing, true)
	default: // down
		vector.FillCircle(img, cx-9, cy+5, 8, wing, true)
	}

	// Eye.
	vector.FillCircle(img, cx+8, cy-6, 4.5, eyeDark, true)
	vector.FillCircle(img, cx+10, cy-7, 1.6, eyeDark, true)
	vector.FillCircle(img, cx+11, cy-7.5, 1.4, color.RGBA{255, 255, 255, 220}, true)

	// Beak: two stacked rectangles in front.
	vector.FillRect(img, cx+12, cy-2, 10, 5, beakColor, true)
	vector.FillRect(img, cx+12, cy+3, 8, 3, beakColor, true)

	drawAccessory(img, acc, cx, cy)
	return img
}

// drawAccessory stamps headgear: 0 none, 1 cap, 2 crown, 3 scarf.
func drawAccessory(img *ebiten.Image, acc uint8, cx, cy float32) {
	switch acc % protocol.AccessoryCount {
	case 1: // cap
		capClr := color.RGBA{40, 60, 90, 255}
		vector.FillRect(img, cx-10, cy-16, 20, 6, capClr, true)
		vector.FillRect(img, cx+4, cy-12, 12, 3, capClr, true)
	case 2: // crown
		gold := color.RGBA{255, 217, 51, 255}
		vector.FillRect(img, cx-9, cy-15, 18, 4, gold, true)
		for i := range 3 {
			vector.FillRect(img, cx-9+7*float32(i), cy-20, 4, 6, gold, true)
		}
	case 3: // scarf
		scarf := color.RGBA{200, 60, 60, 255}
		vector.FillRect(img, cx-10, cy+6, 18, 5, scarf, true)
		vector.FillRect(img, cx-14, cy+10, 5, 10, scarf, true)
	}
}

// BirdRotation maps velocity to the classic nose-dive tilt.
func BirdRotation(vy float64) float64 {
	rot := vy / 16.0
	if rot < -0.5 {
		rot = -0.5
	}
	if rot > 1.25 {
		rot = 1.25
	}
	return rot
}