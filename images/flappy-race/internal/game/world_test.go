package game

import (
	"math"
	"testing"

	"flappy-race/internal/protocol"
)

// run simulates the world for n ticks, flapping the given bird ids on the
// ticks listed in flapsAt (tick number → ids). It returns the death tick of
// each bird (0 if alive after the run).
func run(t *testing.T, seed uint64, ids []uint64, flapsAt map[uint64][]uint64) map[uint64]uint64 {
	t.Helper()
	w := NewWorld(seed)
	for _, id := range ids {
		w.AddBird(id, "p", 0, 0)
	}
	deaths := map[uint64]uint64{}
	for tick := uint64(1); tick <= 300; tick++ {
		fl := map[uint64]struct{}{}
		for _, id := range flapsAt[tick] {
			fl[id] = struct{}{}
		}
		w.Step(fl)
		for _, id := range ids {
			if b := w.Birds[id]; b.Dead && deaths[id] == 0 {
				deaths[id] = tick
			}
		}
	}
	return deaths
}

func TestIdleBirdDiesDeterministically(t *testing.T) {
	// Independent reference integration of the same physics: the bird must
	// reach the ground-death line (y + r >= GroundY) after a computable
	// number of ticks.
	y := protocol.SpawnY
	vy := 0.0
	want := 0
	for tick := 1; tick <= 300; tick++ {
		vy = min(vy+protocol.Gravity, protocol.TerminalVel)
		y += vy
		if y+protocol.BirdRadius >= protocol.GroundY {
			want = tick
			break
		}
	}
	if want == 0 || want > 90 {
		t.Fatalf("reference death tick out of sane range: %d", want)
	}
	for _, seed := range []uint64{0, 1, 42, 123456789} {
		got := run(t, seed, []uint64{7}, nil)[7]
		if got != uint64(want) {
			t.Fatalf("seed %d: idle death at tick %d, reference says %d", seed, got, want)
		}
	}
}

func TestFlapApexMatchesClassicFeel(t *testing.T) {
	w := NewWorld(1)
	w.AddBird(1, "p", 0, 0)
	w.Step(map[uint64]struct{}{1: {}}) // exactly one flap
	minY := w.Birds[1].Y
	for range 30 {
		w.Step(nil)
		if y := w.Birds[1].Y; y < minY {
			minY = y
		}
	}
	rise := protocol.SpawnY - minY
	// Reference integration of the same discrete physics: vy is SET on the
	// flap tick and gravity applies within that same tick.
	want := 0.0
	for vy := protocol.FlapImpulse + protocol.Gravity; vy < 0; vy += protocol.Gravity {
		want += -vy
	}
	if rise < want-1 || rise > want+1 {
		t.Fatalf("apex rise %.2f, want %.2f±1", rise, want)
	}
}

func TestFlapSetsVelocity(t *testing.T) {
	w := NewWorld(1)
	w.AddBird(1, "p", 0, 0)
	w.Step(map[uint64]struct{}{1: {}}) // flap
	w.Step(map[uint64]struct{}{1: {}}) // flap again: vy must be SET, not added
	got := w.Birds[1].Vy
	want := protocol.FlapImpulse + protocol.Gravity
	if got != want {
		t.Fatalf("vy after consecutive flaps = %v, want %v (flap must set, not add)", got, want)
	}
}

func TestCeilingClampsWithoutDeath(t *testing.T) {
	w := NewWorld(1)
	w.AddBird(1, "p", 0, 0)
	// Mash the flap for 30 ticks (well before the first pipe arrives at
	// tick ~133): the bird must ride the ceiling, alive.
	for range 30 {
		w.Step(map[uint64]struct{}{1: {}})
	}
	b := w.Birds[1]
	if b.Dead {
		t.Fatal("ceiling ride must not kill")
	}
	if b.Y < protocol.CeilingY {
		t.Fatalf("y = %v below clamp %v", b.Y, protocol.CeilingY)
	}
}

func TestCollisionGrazeAndClear(t *testing.T) {
	// Hand-crafted pipe at the bird's x: gap from 300 to 456 (center 378,
	// height 156). The bird flies straight into the gap.
	mkworld := func() *World {
		w := NewWorld(1)
		w.AddBird(1, "p", 0, 0)
		w.Pipes = []Pipe{{ID: 1, X: protocol.BirdX, GapY: 378, GapH: 156}}
		return w
	}

	// Center of the gap: survives.
	w := mkworld()
	w.Birds[1].Y, w.Birds[1].Vy = 378, 0
	w.Step(nil)
	if w.Birds[1].Dead {
		t.Fatal("gap center must be safe")
	}

	// Half a pixel into the top pipe's corner: dies.
	w = mkworld()
	w.Birds[1].Y, w.Birds[1].Vy = 300-protocol.BirdRadius+0.5, 0
	w.Step(nil)
	if !w.Birds[1].Dead {
		t.Fatal("corner graze (+0.5) must die")
	}

	// Top edge half a pixel clear of the top pipe's bottom: survives.
	w = mkworld()
	w.Birds[1].Y, w.Birds[1].Vy = 300+protocol.BirdRadius+0.5, 0
	w.Step(nil)
	if w.Birds[1].Dead {
		t.Fatal("corner clear (−0.5) must survive")
	}
}

func TestGroundDeath(t *testing.T) {
	w := NewWorld(1)
	w.AddBird(1, "p", 0, 0)
	// Bottom edge exactly on the ground line: one gravity step carries it
	// below, which must die.
	w.Birds[1].Y = protocol.GroundY - protocol.BirdRadius
	w.Step(nil)
	if !w.Birds[1].Dead {
		t.Fatal("below the ground line must die")
	}
}

func TestScoringOnePerPipe(t *testing.T) {
	w := NewWorld(1)
	w.AddBird(1, "p", 0, 0)
	// A crafted pipe whose gap sits exactly at the bird's pinned height;
	// world spawn continues behind it with IDs from 2 on.
	w.Pipes = []Pipe{{ID: 1, X: 200, GapY: 400, GapH: 156}}
	w.nextPipeID = 1
	w.Birds[1].Y, w.Birds[1].Vy = 400, 0

	score := 0
	for range 90 {
		w.Birds[1].Y, w.Birds[1].Vy = 400, 0 // hover in the gap
		w.Step(nil)
		if s := w.Birds[1].Score; s != score {
			if s != score+1 {
				t.Fatalf("score jumped %d → %d", score, s)
			}
			score = s
		}
		if w.Birds[1].Dead {
			t.Fatalf("died on the crafted gap at tick %d", w.Tick)
		}
	}
	if score != 1 {
		t.Fatalf("expected exactly one score, got %d", score)
	}
}

func TestDifficultyRamp(t *testing.T) {
	w := NewWorld(1)
	w.AddBird(1, "p", 0, 0)
	for i := uint64(1); i <= protocol.RampEveryTicks; i++ {
		w.Step(nil)
	}
	if w.Scroll <= protocol.ScrollSpeed {
		t.Fatalf("speed must ramp: %v", w.Scroll)
	}
	if w.Gap >= protocol.PipeGap {
		t.Fatalf("gap must shrink: %v", w.Gap)
	}
	// Ramp applies to NEW pipes only; old pipes keep their gap.
	w2 := NewWorld(1)
	w2.AddBird(1, "p", 0, 0)
	w2.Step(nil)
	if w2.Pipes[0].GapH != protocol.PipeGap {
		t.Fatalf("pipe gap must snapshot at spawn time: %v", w2.Pipes[0].GapH)
	}
}

func TestPipeGenerationDeterminism(t *testing.T) {
	gen := func(seed uint64) []Pipe {
		w := NewWorld(seed)
		w.AddBird(1, "p", 0, 0)
		for range 200 {
			w.Step(nil)
		}
		return w.Pipes
	}
	a, b := gen(42), gen(42)
	if len(a) != len(b) {
		t.Fatalf("same seed, different pipe counts: %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("pipe %d differs: %+v vs %+v", i, a[i], b[i])
		}
	}
	if gen(43)[0].GapY == a[0].GapY { // 1/384-ish chance of flaking
		t.Log("note: different seeds happened to agree on pipe 1")
	}
}

func TestPipeSpacingAndGapBounds(t *testing.T) {
	w := NewWorld(7)
	w.AddBird(1, "p", 0, 0)
	for range 500 {
		w.Step(nil)
	}
	if len(w.Pipes) < 2 {
		t.Fatal("expected multiple pipes")
	}
	lastGap := 0.0
	for i, p := range w.Pipes {
		if p.GapY < protocol.GapYMin || p.GapY > protocol.GapYMax {
			t.Fatalf("pipe %d gap center %v out of bounds", i, p.GapY)
		}
		if d := math.Abs(p.GapY - lastGap); i > 0 && d > protocol.MaxGapDelta {
			t.Fatalf("adjacent gap delta %.1f exceeds %v", d, protocol.MaxGapDelta)
		}
		if p.GapH < protocol.RampGapFloor {
			t.Fatalf("pipe %d gap height %v below floor", i, p.GapH)
		}
		lastGap = p.GapY
		if i > 0 {
			spacing := p.X - w.Pipes[i-1].X // pipes ordered x ASC, moving together
			if math.Abs(spacing-protocol.PipeSpacing) > 1.5 {
				t.Fatalf("spacing %.1f, want %v", spacing, protocol.PipeSpacing)
			}
		}
		if i > 0 && p.ID != w.Pipes[i-1].ID+1 {
			t.Fatalf("pipe ids not strictly increasing at %d", i)
		}
	}
}

func TestPipeDropsAfterPassing(t *testing.T) {
	w := NewWorld(1)
	w.AddBird(1, "p", 0, 0)
	for range 600 {
		w.Step(nil)
	}
	for _, p := range w.Pipes {
		if p.X <= -protocol.PipeWidth {
			t.Fatalf("pipe %d kept at x=%v behind the drop line", p.ID, p.X)
		}
	}
}

func TestResultsTotalOrder(t *testing.T) {
	w := NewWorld(1)
	w.AddBird(1, "ana", 0, 0)
	w.AddBird(2, "bob", 0, 0)
	w.AddBird(3, "cid", 0, 0)
	// bob scores 2 and dies late, ana scores 2 and dies early, cid scores 5.
	w.Birds[1].Score, w.Birds[1].Dead, w.Birds[1].DeathTick = 2, true, 100
	w.Birds[2].Score, w.Birds[2].Dead, w.Birds[2].DeathTick = 2, true, 200
	w.Birds[3].Score, w.Birds[3].Dead, w.Birds[3].DeathTick = 5, true, 50
	// cid alive instead: must beat everyone on equal ground via the cap rule.
	res := w.Results()
	if len(res) != 3 {
		t.Fatalf("want 3 results, got %d", len(res))
	}
	if res[0].ID != 3 || res[0].Rank != 1 {
		t.Fatalf("cid must win: %+v", res[0])
	}
	if res[1].ID != 2 || res[2].ID != 1 {
		t.Fatalf("same score: later death first, got %+v %+v", res[1], res[2])
	}

	// Alive-at-force-finish ranks by the cap tick.
	w2 := NewWorld(1)
	w2.AddBird(1, "ana", 0, 0)
	w2.AddBird(2, "bob", 0, 0)
	w2.Birds[1].Score = 1
	w2.Birds[2].Score = 1
	res = w2.Results() // both alive → cap tick → id asc
	if res[0].ID != 1 || res[1].ID != 2 {
		t.Fatalf("alive ties break by id asc: %+v", res)
	}
}