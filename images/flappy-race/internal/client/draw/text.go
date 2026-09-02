//go:build js && wasm || native

// Package draw contains the Ebitengine rendering layer: procedurally drawn
// themes, bird sprites and HUD panels. Everything is built at runtime from
// vector primitives and the embedded Go font — the project ships no asset
// files at all.
package draw

import (
	"bytes"
	"image/color"
	"math"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"golang.org/x/image/font/gofont/goregular"
)

// Texter renders text from the embedded Go font at arbitrary sizes.
//
// Every label is pre-rendered once into its own image and blitted from then
// on. text.Draw issues one DrawImage per glyph — glyphs are separate images,
// so WebGL cannot batch them — which made per-frame name labels cost
// O(len(name)) draw calls and blew the wasm frame budget on longer names.
// A cached label costs exactly one DrawImage per frame, however long it is.
type Texter struct {
	src   *text.GoTextFaceSource
	mu    sync.Mutex
	faces map[float64]*text.GoTextFace
	// labels holds the pre-rendered images; Draw/DrawShadowed are render-
	// loop only, the mutex just mirrors Face's caution.
	labels map[labelKey]*labelImg
}

// labelKey identifies one pre-rendered label. Every field is comparable.
type labelKey struct {
	s      string
	size   float64
	shadow bool
	clr    color.RGBA
}

// labelImg is a cached label: the composed image plus the layout width,
// which horizontal centering needs on every blit.
type labelImg struct {
	img *ebiten.Image
	w   float64
}

const (
	// labelPad is the transparent margin around a pre-rendered label: room
	// for glyph side bearings (ink may reach outside the layout box) plus
	// the 1.5 px shadow offset.
	labelPad = 4
	// maxLabelImgs bounds the cache. The working set is small (names are
	// capped at 16 runes, ranks at 8, scores in the low hundreds), so the
	// wipe is pure insurance against unbounded key variety.
	maxLabelImgs = 1024
)

func NewText() (*Texter, error) {
	src, err := text.NewGoTextFaceSource(bytes.NewReader(goregular.TTF))
	if err != nil {
		return nil, err
	}
	return &Texter{src: src, faces: map[float64]*text.GoTextFace{}, labels: map[labelKey]*labelImg{}}, nil
}

// Face returns a cached face at the given pixel size.
func (t *Texter) Face(size float64) *text.GoTextFace {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.face(size)
}

func (t *Texter) face(size float64) *text.GoTextFace {
	f, ok := t.faces[size]
	if !ok {
		f = &text.GoTextFace{Source: t.src, Size: size}
		t.faces[size] = f
	}
	return f
}

// Draw draws s with the top-left corner at (x, y) — or centered horizontally
// when center is true. The label is rendered once and blitted thereafter.
func (t *Texter) Draw(dst *ebiten.Image, s string, size float64, x, y float64, clr color.Color, center bool) {
	if s == "" {
		return
	}
	t.blit(t.label(s, size, clr, false), x, y, center, dst)
}

// DrawShadowed draws text with a subtle dark offset for readability. The
// shadow is baked into the cached label, so both layers still cost one
// DrawImage per frame.
func (t *Texter) DrawShadowed(dst *ebiten.Image, s string, size float64, x, y float64, clr color.Color, center bool) {
	if s == "" {
		return
	}
	t.blit(t.label(s, size, clr, true), x, y, center, dst)
}

func (t *Texter) blit(l *labelImg, x, y float64, center bool, dst *ebiten.Image) {
	if center {
		x -= l.w / 2
	}
	opts := &ebiten.DrawImageOptions{}
	opts.GeoM.Translate(x-labelPad, y-labelPad) // the layout box sits at (pad, pad) in the image
	dst.DrawImage(l.img, opts)
}

// label returns the cached label for s, rendering it on first use.
func (t *Texter) label(s string, size float64, clr color.Color, shadow bool) *labelImg {
	c := color.RGBAModel.Convert(clr).(color.RGBA)
	key := labelKey{s: s, size: size, shadow: shadow, clr: c}
	t.mu.Lock()
	defer t.mu.Unlock()
	if l, ok := t.labels[key]; ok {
		return l
	}
	if len(t.labels) >= maxLabelImgs {
		t.labels = map[labelKey]*labelImg{} // re-rendering is a one-frame hiccup, never a leak
	}
	l := t.renderLabel(s, size, c, shadow)
	t.labels[key] = l
	return l
}

// renderLabel draws s into a padded offscreen image exactly where the old
// per-frame path put it: layout box top-left at (labelPad, labelPad), the
// shadow copy first, the fill on top — the same compositing order as the
// two text.Draw calls this replaces, just done once.
func (t *Texter) renderLabel(s string, size float64, clr color.RGBA, shadow bool) *labelImg {
	face := t.face(size)
	w, h := text.Measure(s, face, size)
	img := ebiten.NewImage(int(math.Ceil(w))+2*labelPad+2, int(math.Ceil(h))+2*labelPad+2)
	if shadow {
		opts := &text.DrawOptions{}
		opts.GeoM.Translate(labelPad+1.5, labelPad+1.5)
		opts.ColorScale.ScaleWithColor(color.RGBA{0, 0, 0, 150})
		text.Draw(img, s, face, opts)
	}
	opts := &text.DrawOptions{}
	opts.GeoM.Translate(labelPad, labelPad)
	opts.ColorScale.ScaleWithColor(clr)
	text.Draw(img, s, face, opts)
	return &labelImg{img: img, w: w}
}