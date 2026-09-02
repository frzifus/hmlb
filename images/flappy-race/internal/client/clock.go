// Package client is the Ebitengine game client. The pure logic (clock,
// interpolation, HUD derivation) is build-tag-free so it is unit-testable
// on the host; everything that touches ebiten lives behind
// //go:build js && wasm || native — the browser, or the desktop dev
// client built by `make native`.
package client

import "math"

// Clock estimates the server wall clock from snapshot timestamps: it keeps
// an EMA of the offset (server − local) with jump rejection. Clients never
// count ticks locally — every timer derives from this estimate.
type Clock struct {
	offset     float64
	have       bool
	jumps      int
	localMs    func() int64
}

const (
	emaAlpha       = 0.2
	jumpRejectMs   = 250.0
	jumpsBeforeResync = 30 // ~1 s of rejected samples → the machine slept
)

// NewClock creates a clock reading local time through localMs.
func NewClock(localMs func() int64) *Clock {
	return &Clock{localMs: localMs}
}

// Sample folds one server timestamp into the estimate.
func (c *Clock) Sample(serverNow int64) {
	o := float64(serverNow - c.localMs())
	if !c.have {
		c.offset = o
		c.have = true
		return
	}
	if d := math.Abs(o - c.offset); d > jumpRejectMs {
		// Occasional jitter is noise; a sustained divergence means the local
		// clock moved (laptop sleep) — resync after a second of rejections.
		c.jumps++
		if c.jumps < jumpsBeforeResync {
			return
		}
		c.offset = o
		c.jumps = 0
		return
	}
	c.jumps = 0
	c.offset += (o - c.offset) * emaAlpha
}

// ServerNow returns the estimated server time in ms.
func (c *Clock) ServerNow() int64 {
	if !c.have {
		return c.localMs()
	}
	return c.localMs() + int64(math.Round(c.offset))
}