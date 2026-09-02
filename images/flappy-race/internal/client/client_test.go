package client

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"flappy-race/internal/protocol"
)

// ————— clock —————

func TestClock(t *testing.T) {
	local := int64(1000)
	c := NewClock(func() int64 { return local })

	// First sample: the offset is adopted verbatim.
	c.Sample(1500) // server is 500 ms ahead
	if got := c.ServerNow(); got != 1500 {
		t.Fatalf("first sample: %d, want 1500", got)
	}

	// EMA convergence towards a constant offset.
	for range 40 {
		local += 100
		c.Sample(local + 500)
	}
	local += 100
	if got := c.ServerNow(); got != local+500 {
		t.Fatalf("EMA diverged: %d, want %d", got, local+500)
	}

	// One huge jump is rejected as noise.
	local += 100
	c.Sample(local - 10000)
	if got := c.ServerNow(); got != local+500 {
		t.Fatalf("single jump must be rejected: %d, want %d", got, local+500)
	}

	// Sustained divergence (laptop slept) resyncs after jumpsBeforeResync.
	for range jumpsBeforeResync + 5 {
		local += 1000
		c.Sample(local + 5000)
	}
	local += 1000
	if got := c.ServerNow(); got != local+5000 {
		t.Fatalf("sustained divergence must resync: %d, want %d", got, local+5000)
	}
}

// ————— ring / interpolation —————

func mkSnap(now int64, tick uint64, y, dist float64, pipeX float64) protocol.Snapshot {
	return protocol.Snapshot{
		T: protocol.TSnapshot, Now: now, Tick: tick, St: protocol.StRacing,
		Theme: 2, Seed: 99, Dist: dist,
		Birds: []protocol.BirdSnap{{ID: 1, Name: "me", Y: y, Vy: -1, Score: 3, Pal: 2, Acc: 1}},
		Pipes: []protocol.PipeSnap{{ID: 1, X: pipeX, G: 400, H: 156}},
	}
}

func TestRingInterpolation(t *testing.T) {
	r := NewRing()
	if _, ok := r.Interp(100); ok {
		t.Fatal("empty ring must yield no view")
	}

	r.Push(mkSnap(1000, 10, 100, 1000, 480))
	r.Push(mkSnap(1034, 12, 132, 1034, 459)) // 33 ms later, one tick apart
	v, ok := r.Interp(1017)                  // exactly halfway
	if !ok {
		t.Fatal("interp failed")
	}
	if v.St != protocol.StRacing || v.Theme != 2 || v.Seed != 99 {
		t.Fatalf("state fields must come from the newest snapshot: %+v", v)
	}
	if y := v.Birds[0].Y; y < 115 || y > 117 {
		t.Fatalf("bird y at midpoint = %v, want ~116", y)
	}
	if x := v.Pipes[0].X; x < 469 || x > 470 {
		t.Fatalf("pipe x at midpoint = %v, want ~469.5", x)
	}
	if d := v.Dist; d < 1016 || d > 1018 {
		t.Fatalf("dist at midpoint = %v", d)
	}

	// Beyond the newest snapshot: hold the newest pose.
	v, _ = r.Interp(5000)
	if y := v.Birds[0].Y; y != 132 {
		t.Fatalf("hold must use the newest pose, got %v", y)
	}

	// Before the first snapshot: hold the first pose.
	v, _ = r.Interp(0)
	if y := v.Birds[0].Y; y != 100 {
		t.Fatalf("hold-first pose, got %v", y)
	}
}

func TestRingStaleAndOverflow(t *testing.T) {
	r := NewRing()
	r.Push(mkSnap(2000, 20, 5, 0, 0))
	r.Push(mkSnap(1000, 10, 5, 0, 0)) // stale: ignored
	if newest, _ := r.Newest(); newest.Now != 2000 {
		t.Fatalf("stale snapshot accepted: %+v", newest)
	}
	for i := range protocol.RingLen + 4 {
		r.Push(mkSnap(3000+int64(i), uint64(i), 5, 0, 0))
	}
	if newest, _ := r.Newest(); newest.Now != 3000+protocol.RingLen+3 {
		t.Fatalf("overflow handling wrong: %d", newest.Now)
	}
}

func TestRingHoldsAfterGap(t *testing.T) {
	r := NewRing()
	r.Push(mkSnap(1000, 10, 100, 0, 0))
	r.Push(mkSnap(1000+protocol.SnapGapMs+50, 20, 300, 0, 0)) // large gap
	v, _ := r.Interp(1000 + protocol.SnapGapMs)               // inside the gap
	if y := v.Birds[0].Y; y != 100 {
		t.Fatalf("gap must snap to the older pose, got %v", y)
	}
}

// ————— HUD —————

func hudSnap(st uint8, goAt int64, birds []protocol.BirdSnap) protocol.Snapshot {
	return protocol.Snapshot{T: protocol.TSnapshot, Now: 10000, St: st, GoAt: goAt, Birds: birds}
}

func TestHUDCountdownBoundaries(t *testing.T) {
	self := uint64(1)
	bird := protocol.BirdSnap{ID: 1, Name: "me", Y: 400, Score: 0}
	cases := []struct {
		remaining int64
		want      string
		flash     bool
	}{
		{3000, "3", false},
		{2500, "3", false},
		{2000, "2", false},
		{1001, "2", false},
		{1000, "1", false},
		{1, "1", false},
		{0, "", true},
		{-200, "", true},
	}
	for _, tc := range cases {
		h := DeriveHUD(hudSnap(protocol.StCountdown, 10000+tc.remaining, []protocol.BirdSnap{bird}), 10000, self)
		if h.Countdown != tc.want || h.Go != tc.flash {
			t.Fatalf("remaining %dms: got %q go=%v, want %q go=%v", tc.remaining, h.Countdown, h.Go, tc.want, tc.flash)
		}
	}

	// GO flash lingers 700 ms into racing, then disappears.
	if h := DeriveHUD(hudSnap(protocol.StRacing, 10000, []protocol.BirdSnap{bird}), 10000+300, 1); !h.Go {
		t.Fatal("GO flash must linger after the flip")
	}
	if h := DeriveHUD(hudSnap(protocol.StRacing, 10000, []protocol.BirdSnap{bird}), 10000+701, 1); h.Go {
		t.Fatal("GO flash must end after 700 ms")
	}
}

func TestHUDLeaderboardAndFlags(t *testing.T) {
	birds := []protocol.BirdSnap{
		{ID: 2, Name: "bob", Y: 1, Score: 5},
		{ID: 1, Name: "me", Y: 2, Score: 5},
		{ID: 3, Name: "cid", Y: 3, Score: 2, Dead: true},
		{ID: 4, Name: "eve", Y: 4, Score: 7},
	}
	h := DeriveHUD(hudSnap(protocol.StRacing, 9000, birds), 10000, 1)
	if h.Spect {
		t.Fatal("racer must not be flagged spectating")
	}
	if h.OwnScore != 5 || h.Dead {
		t.Fatalf("own state wrong: score=%d dead=%v", h.OwnScore, h.Dead)
	}
	want := []struct {
		name string
		rank int
	}{
		{"eve", 1}, // alive, 7
		{"me", 2},  // alive, 5, id 2 < bob's 3? no: bob id=2, me id=1…
		{"bob", 3},
		{"cid", 4}, // dead goes last
	}
	for i, w := range want {
		if h.Rows[i].Name != w.name || h.Rows[i].Rank != w.rank {
			t.Fatalf("row %d = %+v, want %s/%d", i, h.Rows[i], w.name, w.rank)
		}
	}
	if !h.Rows[1].Self {
		t.Fatalf("self must be marked: %+v", h.Rows)
	}
	if h.Rows[3].Alive {
		t.Fatal("dead row must not be alive")
	}

	// Spectator: self is connected but flies no bird.
	h = DeriveHUD(hudSnap(protocol.StRacing, 9000, birds), 10000, 99)
	if !h.Spect {
		t.Fatal("no own bird → spectating")
	}
}

// ————— reconnect backoff —————

func TestRetryDelay(t *testing.T) {
	want := []time.Duration{
		500 * time.Millisecond, // 1st failure → half a second
		time.Second,
		2 * time.Second,
		4 * time.Second,
		8 * time.Second, // capped from here on
		8 * time.Second,
		8 * time.Second,
	}
	for attempts, d := range want {
		if got := retryDelay(attempts + 1); got != d {
			t.Fatalf("retryDelay(%d) = %v, want %v", attempts+1, got, d)
		}
	}
}

// ————— start-screen name entry —————

func TestApplyNameInput(t *testing.T) {
	// Typing appends.
	var buf []rune
	buf, sub := applyNameInput(buf, []rune("bob"), false, false)
	if sub != "" || string(buf) != "bob" {
		t.Fatalf("append: buf=%q sub=%q", buf, sub)
	}

	// Backspace drops one rune and never underflows.
	buf, _ = applyNameInput(buf, nil, true, false)
	if string(buf) != "bo" {
		t.Fatalf("backspace: %q", buf)
	}
	if buf, _ = applyNameInput(nil, nil, true, false); len(buf) != 0 {
		t.Fatal("backspace on an empty buffer must be a no-op")
	}

	// Enter on an empty buffer submits nothing.
	if _, sub := applyNameInput(nil, nil, false, true); sub != "" {
		t.Fatal("empty enter must not submit")
	}

	// Enter submits the trimmed name — the server's own rule.
	buf, _ = applyNameInput([]rune(" x "), nil, false, false)
	buf, sub = applyNameInput(buf, nil, false, true)
	if sub != "x" {
		t.Fatalf("submit must trim: %q", sub)
	}
	if len(buf) != 0 {
		t.Fatal("submit must clear the buffer")
	}

	// Whitespace-only is rejected exactly like the server would.
	if _, sub := applyNameInput([]rune("   "), nil, false, true); sub != "" {
		t.Fatal("spaces-only must not submit")
	}

	// The buffer caps at MaxNameLen runes while typing.
	buf = []rune(strings.Repeat("a", protocol.MaxNameLen))
	buf, _ = applyNameInput(buf, []rune("bc"), false, false)
	if len(buf) != protocol.MaxNameLen {
		t.Fatalf("buffer must cap at %d, got %d", protocol.MaxNameLen, len(buf))
	}
}

func TestHTTPBase(t *testing.T) {
	cases := []struct{ in, want string }{
		{"ws://localhost:8080/ws", "http://localhost:8080"},
		{"wss://birds.klimlive.de/ws", "https://birds.klimlive.de"},
		{"ws://h.example", "http://h.example"},
	}
	for _, tc := range cases {
		if got := httpBase(tc.in); got != tc.want {
			t.Fatalf("httpBase(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// ————— protocol round trip through the client's own structs —————

func TestSnapshotJSONCompatible(t *testing.T) {
	// The client must decode exactly what the server marshals.
	in := protocol.Snapshot{
		T: protocol.TSnapshot, Now: 42, St: protocol.StFinished,
		GoAt: 41, Theme: 3, Seed: 7, Dist: 12.5, Wait: 1,
		Birds: []protocol.BirdSnap{{ID: 1, Name: "me", Y: 400.25, Vy: -14.5, Score: 9, Dead: true, Left: true, Flap: true, Pal: 5, Acc: 2}},
		Pipes: []protocol.PipeSnap{{ID: 1, X: 553.75, G: 400, H: 154}},
		Res:   []protocol.ResultSnap{{ID: 1, Name: "me", Score: 9, Rank: 1, Left: true}},
	}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	var out protocol.Snapshot
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	if out.T != in.T || out.Now != in.Now || out.St != in.St || out.Dist != in.Dist ||
		len(out.Birds) != 1 || out.Birds[0] != in.Birds[0] ||
		len(out.Pipes) != 1 || out.Pipes[0] != in.Pipes[0] ||
		len(out.Res) != 1 || out.Res[0] != in.Res[0] {
		t.Fatalf("round trip mismatch:\n in:  %+v\n out: %+v", in, out)
	}
}