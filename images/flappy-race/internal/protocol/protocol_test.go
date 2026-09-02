package protocol

import (
	"encoding/json"
	"fmt"
	"testing"
)

// TestSnapshotSizeBudget guards the wire budget: a full lobby snapshot
// (8 birds, 10 pipes, results) must stay well under 1.6 KB so a 30 Hz
// broadcast stays trivial for the server and the browser.
func TestSnapshotSizeBudget(t *testing.T) {
	snap := Snapshot{
		T: TSnapshot, Now: 1717000000000, Tick: 7231, St: StRacing,
		GoAt: 1717000003000, Theme: 3, Seed: 98765432101234567, Dist: 45678.9,
		Wait: 12,
	}
	for i := range 8 {
		snap.Birds = append(snap.Birds, BirdSnap{
			ID: uint64(i) + 1, Name: fmt.Sprintf("racer%02d", i),
			Y: 321.45, Vy: -14.5, Score: 17, Dead: i%3 == 0, Left: i%5 == 0,
			Flap: i%2 == 0, Pal: uint8(i), Acc: uint8(i % 4),
		})
	}
	for i := range 10 {
		snap.Pipes = append(snap.Pipes, PipeSnap{
			ID: uint64(i) + 1, X: 480.5 + float64(i)*260, G: 355.5, H: 156,
		})
	}
	for i := range 8 {
		snap.Res = append(snap.Res, ResultSnap{
			ID: uint64(i) + 1, Name: fmt.Sprintf("racer%02d", i),
			Score: 20 - i, Rank: i + 1, Left: i%5 == 0,
		})
	}
	b, err := json.Marshal(snap)
	if err != nil {
		t.Fatal(err)
	}
	if len(b) > 1600 {
		t.Fatalf("snapshot too fat: %d bytes (budget 1600): %s", len(b), b)
	}
	t.Logf("full snapshot: %d bytes", len(b))
}

func TestMessageRoundTrip(t *testing.T) {
	// Welcome
	w := WelcomeMsg{T: TWelcome, You: 42, Name: "klim", Bird: Bird{3, 1},
		Now: 1, St: StCountdown, GoAt: 2, Theme: 1, Seed: 7, Spect: true}
	b, _ := json.Marshal(w)
	var w2 WelcomeMsg
	if err := json.Unmarshal(b, &w2); err != nil || w2 != w {
		t.Fatalf("welcome round trip: %+v vs %+v (%v)", w, w2, err)
	}

	// Snapshot with omitempty fields: missing keys decode to zero values.
	var snap Snapshot
	if err := json.Unmarshal([]byte(`{"t":11,"now":5,"st":0,"ds":0}`), &snap); err != nil {
		t.Fatalf("minimal snapshot: %v", err)
	}
	if snap.T != TSnapshot || snap.St != StWaiting || snap.GoAt != 0 || snap.Birds != nil {
		t.Fatalf("minimal snapshot decoded wrong: %+v", snap)
	}

	// Unknown message types must be tolerable (forward compatibility).
	var env Envelope
	if err := json.Unmarshal([]byte(`{"t":99,"x":{"a":1}}`), &env); err != nil {
		t.Fatalf("unknown type must not error: %v", err)
	}

	// Names.
	if n, ok := ValidateName("  klim "); !ok || n != "klim" {
		t.Fatalf("trim failed: %q %v", n, ok)
	}
	if _, ok := ValidateName(""); ok {
		t.Fatal("empty name accepted")
	}
	if _, ok := ValidateName("   "); ok {
		t.Fatal("whitespace name accepted")
	}
	long := ""
	for range MaxNameLen {
		long += "a"
	}
	if _, ok := ValidateName(long); !ok {
		t.Fatal("boundary-length name rejected")
	}
	if _, ok := ValidateName(long + "a"); ok {
		t.Fatal("overlong name accepted")
	}
	if _, ok := ValidateName("bad\nname"); ok {
		t.Fatal("control character in name accepted")
	}

	// Themes and birds.
	for th := Theme(0); th < ThemeCount; th++ {
		if !th.Valid() || th.String() == "unknown" {
			t.Fatalf("theme %d not stringable", uint8(th))
		}
		if _, ok := ThemeFromString(th.String()); !ok {
			t.Fatalf("theme %s not parseable", th)
		}
	}
	if (ThemeCount).Valid() {
		t.Fatal("ThemeCount must not be a valid theme")
	}
	rngA, rngB := uint64(1), uint64(1)
	bird := RandomBird(func() uint64 { rngA = rngA*2 + 1; return rngA }, nil)
	if !bird.Valid() {
		t.Fatalf("random bird invalid: %+v", bird)
	}
	bird = RandomBird(func() uint64 { rngB++; return rngB }, func(Bird) bool { return true })
	if !bird.Valid() {
		t.Fatal("fallback bird invalid")
	}
}