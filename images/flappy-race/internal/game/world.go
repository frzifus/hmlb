// Package game is the authoritative 60 Hz race simulation. It is pure:
// no I/O, no wall clock, no networking — everything is tick-driven, which
// makes the whole game unit-testable without a GUI or a network.
package game

import "flappy-race/internal/protocol"

// BirdState is one racer inside a running world. A bird stays in the world
// (dead) until the race ends so that scores keep being observable.
type BirdState struct {
	ID     uint64
	Name   string
	Pal    uint8
	Acc    uint8
	Y      float64
	Vy     float64
	Score  int
	Dead   bool
	DeathTick uint64
	Left   bool // disconnected mid-race
	LastFlap  uint64 // tick of the last flap; 0 = never
	NextPipeID uint64 // id of the next pipe that will score
}

// Pipe is one gap-in-the-column obstacle. GapY is the gap center, GapH its
// total height (shrinks as the difficulty ramps).
type Pipe struct {
	ID   uint64
	X    float64
	GapY float64
	GapH float64
}

// World holds one race's simulation state.
type World struct {
	Tick   uint64
	Scroll float64 // current scroll speed (ramped)
	Gap    float64 // gap height used for newly spawned pipes (ramped)
	Dist   float64 // total pixels scrolled — the parallax reference

	Pipes []Pipe

	Order []uint64 // bird ids in join order — keeps snapshots deterministic
	Birds map[uint64]*BirdState

	rng         *RNG
	nextPipeID  uint64
	lastGapY    float64
}

// NewWorld creates an empty world for a race. AddBird is called by the hub
// for every racer before the first tick.
func NewWorld(seed uint64) *World {
	return &World{
		Scroll:    protocol.ScrollSpeed,
		Gap:       protocol.PipeGap,
		Birds:     make(map[uint64]*BirdState),
		rng:       NewRNG(seed),
		lastGapY:  protocol.SpawnY, // keeps the first gap humanly followable
	}
}

// AddBird places a racer at spawn height. Used at race start and for late
// joins during the countdown. Adding an existing id is a no-op.
func (w *World) AddBird(id uint64, name string, pal, acc uint8) {
	if _, ok := w.Birds[id]; ok {
		return
	}
	w.Birds[id] = &BirdState{
		ID:         id,
		Name:       name,
		Pal:        pal,
		Acc:        acc,
		Y:          protocol.SpawnY,
		NextPipeID: 1,
	}
	w.Order = append(w.Order, id)
}

// RemoveBird drops a racer entirely. Only valid for countdown leaves —
// a racer that already raced must stay for the results (see KillLeft).
func (w *World) RemoveBird(id uint64) {
	if _, ok := w.Birds[id]; !ok {
		return
	}
	delete(w.Birds, id)
	for i, oid := range w.Order {
		if oid == id {
			w.Order = append(w.Order[:i], w.Order[i+1:]...)
			break
		}
	}
}

// KillLeft marks a live racer as dead because their connection dropped.
func (w *World) KillLeft(id uint64) {
	b, ok := w.Birds[id]
	if !ok || b.Dead {
		return
	}
	b.Dead = true
	b.Left = true
	b.DeathTick = w.Tick
}

// Alive counts racers still flying.
func (w *World) Alive() int {
	n := 0
	for _, b := range w.Birds {
		if !b.Dead {
			n++
		}
	}
	return n
}

// RacerCount counts all birds in the world, dead or alive.
func (w *World) RacerCount() int { return len(w.Birds) }

// IsRacer reports whether id races in this world (dead counts as racing).
func (w *World) IsRacer(id uint64) bool {
	_, ok := w.Birds[id]
	return ok
}

// Step advances exactly one 60 Hz tick. flaps contains the ids that flapped
// since the previous tick; a flap SETS vy to FlapImpulse, so coalescing
// multiple flaps into one tick is harmless by construction.
func (w *World) Step(flaps map[uint64]struct{}) {
	w.Tick++

	// Difficulty ramp: every RampEveryTicks, speed up and tighten new gaps.
	if w.Tick%protocol.RampEveryTicks == 0 {
		w.Scroll = min(w.Scroll*protocol.RampSpeedMult, protocol.RampSpeedCap)
		w.Gap = max(w.Gap-protocol.RampGapShrink, protocol.RampGapFloor)
	}

	// 1. Apply flap impulses, then integrate physics.
	for _, id := range w.Order {
		b := w.Birds[id]
		if b.Dead {
			continue
		}
		if _, ok := flaps[id]; ok {
			b.Vy = protocol.FlapImpulse
			b.LastFlap = w.Tick
		}
		b.Vy = min(b.Vy+protocol.Gravity, protocol.TerminalVel)
		b.Y += b.Vy
		if b.Y < protocol.CeilingY { // clamp, never death
			b.Y = protocol.CeilingY
			if b.Vy < 0 {
				b.Vy = 0
			}
		}
	}

	// 2. Scroll the world, drop fully-passed pipes, spawn ahead.
	for i := range w.Pipes {
		w.Pipes[i].X -= w.Scroll
	}
	w.Dist += w.Scroll
	kept := w.Pipes[:0]
	for _, p := range w.Pipes {
		if p.X > -protocol.PipeWidth {
			kept = append(kept, p)
		}
	}
	w.Pipes = kept
	for {
		nextX := protocol.FirstPipeX
		if len(w.Pipes) > 0 {
			nextX = w.Pipes[len(w.Pipes)-1].X + protocol.PipeSpacing
		}
		if nextX >= float64(protocol.ViewMaxW)+protocol.PipeSpacing {
			break
		}
		w.spawnPipe(nextX)
	}

	// 3. Collisions and ground deaths, then scoring for survivors.
	for _, id := range w.Order {
		b := w.Birds[id]
		if b.Dead {
			continue
		}
		if b.Y+protocol.BirdRadius >= protocol.GroundY {
			w.kill(b)
			continue
		}
		for i := range w.Pipes {
			p := &w.Pipes[i]
			if p.X > protocol.BirdX+protocol.BirdRadius || p.X+protocol.PipeWidth < protocol.BirdX-protocol.BirdRadius {
				continue // not in the bird's x window
			}
			top := p.GapY - p.GapH/2
			bot := p.GapY + p.GapH/2
			if circleRect(protocol.BirdX, b.Y, protocol.BirdRadius, p.X, 0, p.X+protocol.PipeWidth, top) ||
				circleRect(protocol.BirdX, b.Y, protocol.BirdRadius, p.X, bot, p.X+protocol.PipeWidth, protocol.GroundY) {
				w.kill(b)
				break
			}
		}
		if b.Dead {
			continue
		}
		// Scoring: +1 the first time a pipe fully passes the bird.
		for i := range w.Pipes {
			p := &w.Pipes[i]
			if p.ID == b.NextPipeID && p.X+protocol.PipeWidth < protocol.BirdX-protocol.BirdRadius {
				b.Score++
				b.NextPipeID++
			}
		}
	}
}

func (w *World) kill(b *BirdState) {
	b.Dead = true
	b.DeathTick = w.Tick
}

// BirdSnaps returns every racer in join order, dead included, ready for the
// wire. Flap reports a flap within the last FlapAnimTicks ticks.
func (w *World) BirdSnaps() []protocol.BirdSnap {
	out := make([]protocol.BirdSnap, 0, len(w.Order))
	for _, id := range w.Order {
		b := w.Birds[id]
		out = append(out, protocol.BirdSnap{
			ID:    b.ID,
			Name:  b.Name,
			Y:     round2(b.Y),
			Vy:    round2(b.Vy),
			Score: b.Score,
			Dead:  b.Dead,
			Left:  b.Left,
			Flap:  b.LastFlap != 0 && w.Tick-b.LastFlap < protocol.FlapAnimTicks,
			Pal:   b.Pal,
			Acc:   b.Acc,
		})
	}
	return out
}

// PipeSnaps returns the pipes worth sending: everything the widest
// supported view could show plus one spacing of margin, in x-ascending
// order (which is spawn order).
func (w *World) PipeSnaps() []protocol.PipeSnap {
	visibleUntil := float64(protocol.ViewMaxW) + 2*protocol.PipeSpacing
	out := make([]protocol.PipeSnap, 0, 4)
	for i := range w.Pipes {
		p := &w.Pipes[i]
		if p.X <= -protocol.PipeWidth {
			continue
		}
		if p.X >= visibleUntil {
			break
		}
		out = append(out, protocol.PipeSnap{
			ID: p.ID,
			X:  round2(p.X),
			G:  round2(p.GapY),
			H:  round2(p.GapH),
		})
	}
	return out
}

func (w *World) spawnPipe(x float64) {
	g := w.rng.Range(protocol.GapYMin, protocol.GapYMax)
	// Adjacent gaps stay humanly followable: |Δ| ≤ MaxGapDelta …
	g = clamp(g, w.lastGapY-protocol.MaxGapDelta, w.lastGapY+protocol.MaxGapDelta)
	// … and within the playfield.
	g = clamp(g, protocol.GapYMin, protocol.GapYMax)
	w.lastGapY = g
	w.nextPipeID++
	w.Pipes = append(w.Pipes, Pipe{ID: w.nextPipeID, X: x, GapY: g, GapH: w.Gap})
}

// circleRect reports whether a circle intersects an axis-aligned rectangle,
// using the exact closest-point test so corner grazes are fair.
func circleRect(cx, cy, r, x0, y0, x1, y1 float64) bool {
	px := clamp(cx, x0, x1)
	py := clamp(cy, y0, y1)
	dx := cx - px
	dy := cy - py
	return dx*dx+dy*dy < r*r
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// round2 keeps snapshots small and deterministic; sub-0.01 px is invisible.
func round2(v float64) float64 { return float64(int64(v*100+0.5)) / 100 }