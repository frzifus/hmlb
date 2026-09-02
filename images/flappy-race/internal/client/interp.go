package client

import (
	"flappy-race/internal/protocol"
)

// BirdView is one bird's rendered state: identity/flags from the newest
// snapshot (authoritative), positions interpolated between snapshots.
type BirdView struct {
	ID    uint64
	Name  string
	Y, Vy float64
	Score int
	Dead  bool
	Left  bool
	Flap  bool
	Pal   uint8
	Acc   uint8
}

// PipeView is one pipe's rendered state.
type PipeView struct {
	ID    uint64
	X     float64
	GapY  float64
	GapH  float64
}

// View is the interpolated world state for one rendered frame.
type View struct {
	St    uint8
	Theme uint8
	Seed  uint64
	GoAt  int64
	Dist  float64
	Wait  int

	Birds []BirdView
	Pipes []PipeView
	Res   []protocol.ResultSnap
}

// Ring keeps the last protocol.RingLen snapshots and turns them into an
// interpolated View at a render time that trails server time by
// protocol.InterpDelayMs. It owns its output buffers (reused every call —
// the render loop runs at 60 fps and must not allocate per frame).
type Ring struct {
	snaps []protocol.Snapshot
	birds []BirdView
	pipes []PipeView
}

func NewRing() *Ring {
	return &Ring{snaps: make([]protocol.Snapshot, 0, protocol.RingLen)}
}

// Push inserts a snapshot, dropping the oldest when full and ignoring
// out-of-order frames.
func (r *Ring) Push(s protocol.Snapshot) {
	if len(r.snaps) > 0 && s.Now <= r.snaps[len(r.snaps)-1].Now {
		return // stale
	}
	r.snaps = append(r.snaps, s)
	if len(r.snaps) > protocol.RingLen {
		copy(r.snaps, r.snaps[1:])
		r.snaps = r.snaps[:protocol.RingLen]
	}
}

// Reset drops all buffered snapshots. Used on reconnect: a fresh server
// epoch must not be interpolated against the old one, and the out-of-order
// guard in Push must not reject a restarted server whose clock may read
// older than the snapshots gathered before the drop.
func (r *Ring) Reset() { r.snaps = r.snaps[:0] }

// Newest returns the latest snapshot without interpolation.
func (r *Ring) Newest() (protocol.Snapshot, bool) {
	if len(r.snaps) == 0 {
		return protocol.Snapshot{}, false
	}
	return r.snaps[len(r.snaps)-1], true
}

// Interp renders the world as it looked at renderT (server ms), producing
// smooth 60 fps positions out of 30 Hz snapshots.
func (r *Ring) Interp(renderT int64) (View, bool) {
	if len(r.snaps) == 0 {
		return View{}, false
	}

	// Find the bracket: s0 = last snapshot at or before renderT,
	// s1 = first after it. Without a bracket, hold the closest pose.
	s0, s1 := &r.snaps[0], &r.snaps[0]
	alpha := 1.0 // 0 → s0 pose, 1 → s1 pose
	for i := range r.snaps {
		if r.snaps[i].Now <= renderT {
			s0 = &r.snaps[i]
			s1 = &r.snaps[i]
			if i+1 < len(r.snaps) {
				s1 = &r.snaps[i+1]
			}
			continue
		}
		break
	}
	if s0 != s1 {
		span := s1.Now - s0.Now
		if span > protocol.SnapGapMs {
			// Snapshot gap too large (hitch/loss): snap instead of lerp.
			s1 = s0
			alpha = 0
		} else if span > 0 {
			alpha = float64(renderT-s0.Now) / float64(span)
		} else {
			alpha = 0
		}
	}

	v := View{
		St:    s1.St,
		Theme: s1.Theme,
		Seed:  s1.Seed,
		GoAt:  s1.GoAt,
		Wait:  s1.Wait,
		Res:   s1.Res,
	}
	v.Dist = lerp(s0.Dist, s1.Dist, alpha)

	r.birds = r.birds[:0]
	for _, b := range s1.Birds {
		bv := BirdView{
			ID: b.ID, Name: b.Name, Score: b.Score,
			Dead: b.Dead, Left: b.Left, Flap: b.Flap, Pal: b.Pal, Acc: b.Acc,
		}
		if o, ok := findBird(*s0, b.ID); ok {
			bv.Y, bv.Vy = lerp(o.Y, b.Y, alpha), lerp(o.Vy, b.Vy, alpha)
		} else {
			bv.Y, bv.Vy = b.Y, b.Vy
		}
		r.birds = append(r.birds, bv)
	}
	v.Birds = r.birds

	r.pipes = r.pipes[:0]
	for _, p := range s1.Pipes {
		pv := PipeView{ID: p.ID, GapY: p.G, GapH: p.H}
		if o, ok := findPipe(*s0, p.ID); ok {
			pv.X = lerp(o.X, p.X, alpha)
		} else {
			pv.X = p.X
		}
		r.pipes = append(r.pipes, pv)
	}
	v.Pipes = r.pipes
	return v, true
}

func findBird(s protocol.Snapshot, id uint64) (protocol.BirdSnap, bool) {
	for _, b := range s.Birds {
		if b.ID == id {
			return b, true
		}
	}
	return protocol.BirdSnap{}, false
}

func findPipe(s protocol.Snapshot, id uint64) (protocol.PipeSnap, bool) {
	for _, p := range s.Pipes {
		if p.ID == id {
			return p, true
		}
	}
	return protocol.PipeSnap{}, false
}

func lerp(a, b, t float64) float64 { return a + (b-a)*t }