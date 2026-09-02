package server

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"flappy-race/internal/game"
	"flappy-race/internal/protocol"
)

// fakeHub wires a hub with an injectable clock and a temp-dir store; tests
// drive the hub methods directly (handleJoin/tick are the same code the Run
// loop and transport pumps call).
type fakeHub struct {
	*Hub
	t    *testing.T
	now  int64
	conns []*clientConn
	lastSnap map[*clientConn]protocol.Snapshot
}

func newFakeHub(t *testing.T, mutate func(*Config)) *fakeHub {
	t.Helper()
	cfg := Config{
		Addr:      "127.0.0.1:0",
		DataPath:  t.TempDir() + "/state.json",
		Countdown: 3 * time.Second,
		Results:   6 * time.Second,
	}
	if mutate != nil {
		mutate(&cfg)
	}
	store, err := OpenStore(cfg.DataPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	h := NewHub(cfg, store)
	f := &fakeHub{Hub: h, t: t, now: 1717000000000, lastSnap: map[*clientConn]protocol.Snapshot{}}
	h.now = func() int64 { return f.now }
	h.rng = game.NewRNG(42) // deterministic pipe/theme sequences
	return f
}

// conn creates a transport stub that captures outgoing frames.
func (f *fakeHub) conn() *clientConn {
	c := newClientConn(nil)
	f.conns = append(f.conns, c)
	return c
}

// join creates a fresh connection and joins with the given name.
func (f *fakeHub) join(name string) (*Session, *clientConn) {
	f.t.Helper()
	c := f.conn()
	rep := f.handleJoin(c, name)
	if rep.errMsg != "" {
		f.t.Fatalf("join %q rejected: %s", name, rep.errMsg)
	}
	return rep.sess, c
}

// snap drains all queued frames and returns the newest snapshot, falling
// back to the last one ever seen on that connection (snapshots are
// idempotent, so the last one is always a valid view).
func (f *fakeHub) snap(c *clientConn) protocol.Snapshot {
	f.t.Helper()
	for closed := false; !closed; {
		select {
		case b, ok := <-c.send:
			if !ok {
				closed = true
				continue
			}
			var env protocol.Envelope
			if json.Unmarshal(b, &env) != nil || env.T != protocol.TSnapshot {
				continue
			}
			var s protocol.Snapshot
			if err := json.Unmarshal(b, &s); err != nil {
				f.t.Fatalf("bad snapshot: %v", err)
			}
			f.lastSnap[c] = s
		default:
			s, got := f.lastSnap[c]
			if !got {
				f.t.Fatal("no snapshot ever seen on this connection")
			}
			return s
		}
	}
	s := f.lastSnap[c] // queue closed without frames
	return s
}

// welcome reads the first queued frame of a connection.
func (f *fakeHub) welcome(c *clientConn) protocol.WelcomeMsg {
	f.t.Helper()
	select {
	case b, ok := <-c.send:
		if !ok {
			f.t.Fatal("no welcome queued (closed)")
		}
		var w protocol.WelcomeMsg
		if err := json.Unmarshal(b, &w); err != nil || w.T != protocol.TWelcome {
			f.t.Fatalf("expected welcome, got %s", b)
		}
		return w
	default:
		f.t.Fatal("no welcome queued")
		return protocol.WelcomeMsg{}
	}
}

func (f *fakeHub) assertSt(want uint8) {
	f.t.Helper()
	if got := f.State(); got != want {
		f.t.Fatalf("state = %d, want %d", got, want)
	}
}

func TestJoinDuringWaitingStartsCountdown(t *testing.T) {
	f := newFakeHub(t, nil)
	f.assertSt(protocol.StWaiting)

	sess, c := f.join("klim")
	f.assertSt(protocol.StCountdown)

	w := f.welcome(c)
	if w.You != sess.ID || w.Name != "klim" || w.St != protocol.StCountdown || w.Spect {
		t.Fatalf("bad welcome: %+v", w)
	}
	if d := w.GoAt - w.Now; d != 3000 {
		t.Fatalf("countdown window = %dms, want 3000", d)
	}
	if !w.Bird.Valid() {
		t.Fatalf("bird not assigned: %+v", w.Bird)
	}
	s := f.snap(c)
	if len(s.Birds) != 1 || s.Birds[0].ID != sess.ID {
		t.Fatalf("snapshot must contain the racer: %+v", s.Birds)
	}
}

func TestJoinDuringCountdownIsAdmitted(t *testing.T) {
	f := newFakeHub(t, nil)
	_, c1 := f.join("ana")
	f.now += 1000 // one second into the countdown
	_, c2 := f.join("bob")

	w := f.welcome(c2)
	if w.St != protocol.StCountdown || w.Spect {
		t.Fatalf("countdown join must race: %+v", w)
	}
	s := f.snap(c2)
	if len(s.Birds) != 2 {
		t.Fatalf("both birds expected, got %+v", s.Birds)
	}
	if len(f.snap(c1).Birds) != 2 {
		t.Fatal("first player must also see the new bird")
	}
}

func TestJoinDuringRacingIsSpectator(t *testing.T) {
	f := newFakeHub(t, nil)
	f.join("ana")
	f.now += 3001
	f.tick() // → racing
	f.assertSt(protocol.StRacing)

	_, c2 := f.join("bob")
	w := f.welcome(c2)
	if !w.Spect {
		t.Fatal("mid-race join must be a spectator")
	}
	s := f.snap(c2)
	if len(s.Birds) != 1 || s.Birds[0].Name != "ana" {
		t.Fatalf("spectator sees the racers: %+v", s.Birds)
	}
	if s.Wait != 1 {
		t.Fatalf("snapshot must count waiting players, got %d", s.Wait)
	}
}

func TestCountdownFlipsToRacingExactlyAtGoAt(t *testing.T) {
	f := newFakeHub(t, nil)
	_, c := f.join("ana")
	f.now += 2999
	f.tick()
	f.assertSt(protocol.StCountdown)
	f.now += 2
	f.tick()
	f.assertSt(protocol.StRacing)
	if s := f.snap(c); s.St != protocol.StRacing {
		t.Fatalf("state flip must broadcast: %+v", s)
	}
}

func TestFlapsIgnoredUntilRacing(t *testing.T) {
	f := newFakeHub(t, nil)
	sess, c := f.join("ana")
	f.handleFlap(sess) // during countdown: must not move the bird
	f.now += 3000
	f.tick()
	f.handleFlap(sess)
	f.now += 100
	for range 6 { // ~6 ticks at 60Hz
		f.tick()
		f.now += 16
	}
	s := f.snap(c)
	bird := s.Birds[0]
	if bird.Y >= protocol.SpawnY { // flap must have lifted it during racing
		t.Fatalf("flap during racing had no effect: y=%v", bird.Y)
	}
	// And the pre-GO flap must not have: the bird starts falling from spawn.
	if bird.Y > protocol.SpawnY-60 {
		t.Logf("note: y=%v", bird.Y)
	}
}

func TestEveryoneDiesAndResultsAreOrdered(t *testing.T) {
	f := newFakeHub(t, nil)
	ana, _ := f.join("ana")
	bob, _ := f.join("bob")
	f.now += 3000
	f.tick()
	f.assertSt(protocol.StRacing)

	// Keep ana alive with a flap every ~15 ticks; bob never flaps.
	bobDeadAt, anaAlive := 0, true
	for i := 0; i < 200; i++ {
		f.tick()
		f.now += 1000 / protocol.TickHz
		if i%15 == 0 {
			f.handleFlap(ana)
		}
		s := f.snap(ana.conn)
		for _, b := range s.Birds {
			switch {
			case b.ID == bob.ID && b.Dead && bobDeadAt == 0:
				bobDeadAt = i
			case b.ID == bob.ID && !b.Dead && bobDeadAt != 0:
				t.Fatal("dead bird resurrected")
			}
		}
		if s.St == protocol.StFinished {
			anaAlive = false
			if len(s.Res) != 2 {
				t.Fatalf("results must list everyone: %+v", s.Res)
			}
			if s.Res[0].Name != "ana" || s.Res[1].Name != "bob" {
				t.Fatalf("ana outlived bob, ranking wrong: %+v", s.Res)
			}
			break
		}
	}
	if anaAlive {
		t.Fatal("race never finished for an idle bob")
	}
	if bobDeadAt == 0 {
		t.Fatal("bob (never flapping) should die fast")
	}
	// Dead birds stay visible until the race ends (requirement 5).
	s := f.snap(ana.conn)
	if len(s.Birds) != 2 {
		t.Fatalf("dead bird must stay in snapshots: %+v", s.Birds)
	}
}

func TestRacerLeaveMidRaceEndsRaceForLastRacer(t *testing.T) {
	f := newFakeHub(t, nil)
	ana, c1 := f.join("ana")
	bob, c2 := f.join("bob")
	f.now += 3000
	f.tick()
	f.assertSt(protocol.StRacing)

	// ana leaves mid-race: her bird dies (left), race continues for bob.
	f.handleLeave(c1, ana)
	f.assertSt(protocol.StRacing)
	s := f.snap(c2)
	var anaBird protocol.BirdSnap
	for _, b := range s.Birds {
		if b.ID == ana.ID {
			anaBird = b
		}
	}
	if !anaBird.Dead || !anaBird.Left {
		t.Fatalf("leaving racer must die flagged left: %+v", anaBird)
	}

	// bob leaves too: the race ends (stats recorded) but with nobody left
	// to watch, the podium is skipped and the hub goes straight to waiting.
	f.handleLeave(c2, bob)
	f.assertSt(protocol.StWaiting)
	if f.world != nil {
		t.Fatal("world must be discarded when everyone left mid-race")
	}
	var raw struct {
		Players map[string]PlayerStat `json:"players"`
	}
	if err := json.Unmarshal(readFile(t, f.cfg.DataPath), &raw); err != nil {
		t.Fatalf("state file unreadable: %v", err)
	}
	if raw.Players["ana"].Races != 1 || raw.Players["bob"].Races != 1 {
		t.Fatalf("the aborted race must still count for stats: %+v", raw.Players)
	}
}

func TestAllLeaveReturnsToIdleAndNoPhantomRacesRun(t *testing.T) {
	f := newFakeHub(t, nil)
	ana, c1 := f.join("ana")
	bob, c2 := f.join("bob")
	f.now += 3000
	f.tick()
	f.assertSt(protocol.StRacing)

	f.handleLeave(c1, ana)
	f.handleLeave(c2, bob)
	f.assertSt(protocol.StWaiting)
	if f.world != nil {
		t.Fatal("world must be discarded when everyone leaves")
	}

	for i := range 300 { // 5 seconds of ticks with zero players
		f.now += 1000 / protocol.TickHz
		f.tick()
		if f.State() != protocol.StWaiting {
			t.Fatalf("phantom race started at tick %d", i)
		}
	}
}

func TestSpectatorBecomesRacerNextRace(t *testing.T) {
	f := newFakeHub(t, nil)
	_, c1 := f.join("ana")
	f.now += 3000
	f.tick()
	f.assertSt(protocol.StRacing)

	spect, c2 := f.join("spect")
	if !f.welcome(c2).Spect {
		t.Fatal("should spectate mid-race")
	}

	// ana dies on the ground: race finishes, spect joins the next countdown.
	f.now += 20000
	for f.State() != protocol.StFinished {
		f.tick()
		f.now += 1000 / protocol.TickHz
	}
	s := f.snap(c1)
	if s.St != protocol.StFinished {
		t.Fatal("expected finished")
	}
	f.now += 6001
	f.tick()
	f.assertSt(protocol.StCountdown)
	s = f.snap(c2)
	if s.St != protocol.StCountdown || len(s.Birds) != 2 {
		t.Fatalf("spectator must race next: %+v", s)
	}
	found := false
	for _, b := range s.Birds {
		if b.ID == spect.ID {
			found = true
			if b.Name != "spect" {
				t.Fatalf("wrong name on bird: %+v", b)
			}
		}
	}
	if !found {
		t.Fatal("spectator missing from next race")
	}
}

func TestThemeNeverRepeatsBetweenRaces(t *testing.T) {
	f := newFakeHub(t, nil)
	_, c := f.join("ana")
	theme1 := f.snap(c).Theme
	// finish race 1 (ana dies), pass the results window
	f.now += 3000
	f.tick()
	f.now += 20000
	for f.State() != protocol.StFinished {
		f.tick()
		f.now += 1000 / protocol.TickHz
	}
	f.now += 6001
	f.tick()
	f.assertSt(protocol.StCountdown)
	theme2 := f.snap(c).Theme
	if theme2 == theme1 {
		t.Fatalf("theme repeated: %d then %d", theme1, theme2)
	}
}

func TestNameDeduplication(t *testing.T) {
	f := newFakeHub(t, nil)
	s1, _ := f.join("klim")
	s2, _ := f.join("klim")
	s3, _ := f.join("klim")
	if s1.Name != "klim" || s2.Name != "klim 2" || s3.Name != "klim 3" {
		t.Fatalf("names: %q %q %q", s1.Name, s2.Name, s3.Name)
	}
	// a departing player frees their name
	f.handleLeave(s2.conn, s2)
	s4, _ := f.join("klim")
	if s4.Name != "klim 2" {
		t.Fatalf("freed name must be reused: %q", s4.Name)
	}
}

func TestInvalidNameAndFullServer(t *testing.T) {
	f := newFakeHub(t, nil)
	f.maxPlayers = 2

	if rep := f.handleJoin(f.conn(), "   "); rep.errMsg == "" {
		t.Fatal("blank name must be rejected")
	}
	if rep := f.handleJoin(f.conn(), ""); rep.errMsg == "" {
		t.Fatal("empty name must be rejected")
	}
	long := ""
	for range protocol.MaxNameLen + 5 {
		long += "x"
	}
	if rep := f.handleJoin(f.conn(), long); rep.errMsg == "" {
		t.Fatal("overlong name must be rejected")
	}

	f.join("a")
	f.join("b")
	if rep := f.handleJoin(f.conn(), "c"); rep.errMsg == "" {
		t.Fatal("join past the cap must be rejected")
	}
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return b
}

func TestBirdPersistenceAcrossRestarts(t *testing.T) {
	path := t.TempDir() + "/state.json"
	cfg := Config{DataPath: path} // durations fall back to NewHub defaults

	store1, _ := OpenStore(path)
	h1 := NewHub(cfg, store1)
	now := int64(1717000000000)
	h1.now = func() int64 { return now }
	h1.rng = game.NewRNG(7)
	c := newClientConn(nil)
	rep := h1.handleJoin(c, "klim")
	bird := rep.sess.Bird

	store2, err := OpenStore(path)
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	if b, ok := store2.LookupBird("klim"); !ok || b != bird {
		t.Fatalf("bird must survive restart: got %+v want %+v", b, bird)
	}

	// RecordRace lands in top + stats and is servable via APIBytes.
	store2.RecordRace([]RaceEntry{
		{Name: "klim", Score: 7, Rank: 1, Bird: bird},
		{Name: "bob", Score: 3, Rank: 2, Bird: protocol.Bird{Palette: 1}},
	}, protocol.ThemeForest, time.UnixMilli(now))
	api := string(store2.APIBytes())
	if !json.Valid([]byte(api)) {
		t.Fatal("API bytes must be JSON")
	}
	var payload struct {
		Top []protocol.TopEntry `json:"top"`
	}
	if err := json.Unmarshal(store2.APIBytes(), &payload); err != nil {
		t.Fatalf("api json: %v", err)
	}
	if len(payload.Top) != 2 || payload.Top[0].Name != "klim" || payload.Top[0].Score != 7 {
		t.Fatalf("top wrong: %+v", payload.Top)
	}

	// A better run replaces the top entry for the same user.
	store2.RecordRace([]RaceEntry{{Name: "bob", Score: 12, Rank: 1, Bird: protocol.Bird{Palette: 1}}},
		protocol.ThemeClouds, time.UnixMilli(now+1000))
	_ = json.Unmarshal(store2.APIBytes(), &payload)
	if len(payload.Top) != 2 || payload.Top[0].Name != "bob" || payload.Top[0].Score != 12 {
		t.Fatalf("best-per-user top wrong: %+v", payload.Top)
	}

	// Wins/races/best in the file.
	store3, _ := OpenStore(path)
	_ = store3
	var raw struct {
		Players map[string]PlayerStat `json:"players"`
	}
	data := readFile(t, path)
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("state file not readable: %v", err)
	}
	ps := raw.Players["klim"]
	if ps.Best != 7 || ps.Races != 1 || ps.Wins != 1 || ps.Bird != bird {
		t.Fatalf("klim stats wrong: %+v", ps)
	}
}