package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"flappy-race/internal/game"
	"flappy-race/internal/protocol"
)

// event kinds queued for the hub goroutine.
const (
	evJoin = iota
	evFlap
	evLeave
)

type event struct {
	kind  int
	conn  *clientConn
	sess  *Session
	name  string
	reply chan joinReply
}

type joinReply struct {
	sess    *Session
	errMsg  string
}

// raceInfo remembers each racer's display data for the lifetime of a race,
// even after they disconnect (their bird stays in the world and the
// results must still show a name).
type raceInfo struct {
	name string
	bird protocol.Bird
}

// Hub owns ALL game state: the player registry, the race state machine and
// the authoritative 60 Hz world. Exactly one goroutine (Run, or the test
// driver) mutates it; transport goroutines communicate only via events.
//
//	State machine (wire states in protocol):
//	  waiting → countdown → racing → finished → countdown (new theme) …
//	  waiting ⇐ last player leaves at any point
type Hub struct {
	cfg   Config
	log   *slog.Logger
	store *Store
	now   func() int64 // wall clock in ms (injectable for tests)
	rng   *game.RNG

	events chan event
	tickC  chan struct{} // ticker signal in Run

	players map[uint64]*Session
	order   []uint64 // session ids in join order (deterministic broadcasts)
	names   map[string]uint64
	nextID  uint64

	st       uint8
	goAt     int64 // ms: countdown target / race start
	finishAt int64 // ms: results window ends
	lastSnap int64 // ms: last snapshot broadcast

	world    *game.World
	raceInfo map[uint64]raceInfo
	results  []protocol.ResultSnap
	theme    protocol.Theme
	seed     uint64
	lastTheme protocol.Theme

	pendingFlaps map[uint64]struct{}

	snapshotEvery int64 // ms between snapshots
	maxPlayers    int
}

// NewHub creates a hub. Tests may override now/rng through the returned
// struct directly before Run starts.
func NewHub(cfg Config, store *Store) *Hub {
	if cfg.Countdown <= 0 {
		cfg.Countdown = 3 * time.Second
	}
	if cfg.Results <= 0 {
		cfg.Results = 6 * time.Second
	}
	snapshotEvery := int64(1000 / protocol.SnapshotHz)
	if cfg.SnapshotHz > 0 {
		snapshotEvery = int64(1000 / cfg.SnapshotHz)
	}
	maxPlayers := protocol.MaxPlayers
	if cfg.MaxPlayers > 0 {
		maxPlayers = cfg.MaxPlayers
	}
	return &Hub{
		cfg:           cfg,
		log:           slog.Default(),
		store:         store,
		now:           func() int64 { return time.Now().UnixMilli() },
		rng:           game.NewRNG(uint64(time.Now().UnixNano())),
		events:        make(chan event, 128),
		tickC:         make(chan struct{}, 1),
		players:       map[uint64]*Session{},
		names:         map[string]uint64{},
		world:         nil,
		theme:         0,
		pendingFlaps:  map[uint64]struct{}{},
		snapshotEvery: snapshotEvery,
		maxPlayers:    maxPlayers,
	}
}

// Run drives the hub until ctx is cancelled: it processes transport events
// and simulates a 60 Hz tick. Tests drive the same methods directly
// (dispatch/tick) instead of running this loop.
func (h *Hub) Run(ctx context.Context) {
	ticker := time.NewTicker(time.Second / protocol.TickHz)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			h.shutdown()
			return
		case ev := <-h.events:
			h.dispatch(ev)
		case <-ticker.C:
			h.tick()
		}
	}
}

// shutdown closes every player's queue; the writer goroutines flush and
// close the sockets.
func (h *Hub) shutdown() {
	for _, s := range h.players {
		s.conn.close()
	}
}

// — transport-facing API (called from connection goroutines) —

// join registers a new connection. It blocks until the hub has processed
// the request (the hub goroutine is the only state mutator). Either sess is
// set, or errMsg explains why the join was refused — the hub has already
// queued the error frame and closed the transport in that case.
func (h *Hub) join(c *clientConn, name string) (*Session, string) {
	ev := event{kind: evJoin, conn: c, name: name, reply: make(chan joinReply, 1)}
	h.submit(ev)
	r := <-ev.reply
	return r.sess, r.errMsg
}

// flap forwards an input impulse. Fire-and-forget: a full event queue
// (server overloaded) drops the flap, which is the correct behavior for a
// game input that merely sets velocity.
func (h *Hub) flap(sess *Session) {
	h.submit(event{kind: evFlap, sess: sess})
}

// leave notifies the hub that a connection ended. sess is nil if the
// connection dropped before completing its join.
func (h *Hub) leave(c *clientConn, sess *Session) {
	h.submit(event{kind: evLeave, conn: c, sess: sess})
}

func (h *Hub) submit(ev event) {
	select {
	case h.events <- ev:
	default:
		// the hub is wedged or flooded; for joins and leaves fall back to
		// a blocking send — they MUST NOT be lost.
		if ev.kind != evFlap {
			select {
			case h.events <- ev:
			case <-time.After(2 * time.Second):
			}
		}
	}
}

// dispatch processes one event; exposed for tests, called from Run.
func (h *Hub) dispatch(ev event) {
	switch ev.kind {
	case evJoin:
		ev.reply <- h.handleJoin(ev.conn, ev.name)
	case evFlap:
		h.handleFlap(ev.sess)
	case evLeave:
		h.handleLeave(ev.conn, ev.sess)
	}
}

// — state machine (hub goroutine only) —

// handleJoin validates the username, assigns id + bird, admits the player
// to the lobby or current race, and sends the welcome frame.
func (h *Hub) handleJoin(c *clientConn, name string) joinReply {
	if len(h.players) >= h.maxPlayers {
		h.reject(c, "server full")
		return joinReply{errMsg: "server full"}
	}
	name, ok := protocol.ValidateName(name)
	if !ok {
		h.reject(c, "please pick a name (1-16 characters)")
		return joinReply{errMsg: "invalid name"}
	}
	name = h.uniqueName(name)

	id := h.nextID + 1
	h.nextID = id

	bird, known := h.store.LookupBird(name)
	if !known {
		bird = protocol.RandomBird(h.rng.Next, h.birdTaken)
		h.store.RememberBird(name, bird)
	}

	sess := &Session{ID: id, Name: name, Bird: bird, conn: c}
	h.players[id] = sess
	h.names[name] = id
	h.order = append(h.order, id)

	now := h.now()
	switch h.st {
	case protocol.StWaiting:
		h.beginCountdown(now) // the first connection starts the race cycle
	case protocol.StCountdown:
		h.admitRacer(sess) // late joiner: still races if before the GO instant
	default: // racing or finished: spectator until the next race
	}

	// Welcome strictly before the first snapshot so clients can order frames.
	h.sendWelcome(sess, now)
	// Everyone (including the newcomer) gets the updated world immediately.
	h.broadcast()
	return joinReply{sess: sess}
}

// handleFlap applies rate limiting and queues the impulse for the next
// simulation tick. Flaps outside a race are ignored by construction.
func (h *Hub) handleFlap(sess *Session) {
	if h.st != protocol.StRacing || sess == nil {
		return
	}
	sec := h.now() / 1000
	if sess.flapSec != sec {
		sess.flapSec, sess.flapCount = sec, 0
	}
	if sess.flapCount >= protocol.FlapRateLimit {
		return
	}
	sess.flapCount++
	if !h.world.IsRacer(sess.ID) {
		return
	}
	h.pendingFlaps[sess.ID] = struct{}{}
}

// handleLeave removes a player and keeps the race coherent: a racer that
// leaves mid-race dies (kept in the results), and the race ends early when
// nobody is left to fly it. The leaver's queue is closed LAST — the state
// machine may still broadcast, and the leaver is already gone from the
// player map by then, so no enqueue can touch their closed queue.
func (h *Hub) handleLeave(c *clientConn, sess *Session) {
	now := h.now()
	if sess == nil {
		if c != nil {
			c.close() // never joined: just stop the writer
		}
		return
	}
	delete(h.players, sess.ID)
	delete(h.names, sess.Name)
	for i, id := range h.order {
		if id == sess.ID {
			h.order = append(h.order[:i], h.order[i+1:]...)
			break
		}
	}

	switch h.st {
	case protocol.StCountdown:
		if h.world != nil && h.world.IsRacer(sess.ID) {
			h.world.RemoveBird(sess.ID)
			delete(h.raceInfo, sess.ID)
			if h.world.RacerCount() == 0 {
				h.st = protocol.StWaiting
				h.world, h.raceInfo, h.results = nil, nil, nil
			} else {
				h.broadcast()
			}
		}
	case protocol.StRacing:
		if h.world.IsRacer(sess.ID) {
			h.world.KillLeft(sess.ID)
			if h.world.Alive() == 0 {
				h.finishRace(now)
			} else {
				h.broadcast()
			}
		}
	case protocol.StFinished:
		if len(h.players) == 0 {
			h.st = protocol.StWaiting
			h.world, h.raceInfo, h.results = nil, nil, nil
		}
	case protocol.StWaiting:
		// nothing; waiting means no race exists
	}

	if c != nil {
		c.close() // flush + close the socket, strictly after the broadcasts
	}
}

// tick advances one simulation step; called 60×/s from Run (or by tests).
func (h *Hub) tick() {
	now := h.now()
	switch h.st {
	case protocol.StCountdown:
		if now >= h.goAt {
			h.st = protocol.StRacing
			h.broadcast() // immediate state flip
		} else {
			h.maybeSnapshot(now)
		}
	case protocol.StRacing:
		flaps := h.pendingFlaps
		h.pendingFlaps = make(map[uint64]struct{}, len(flaps))
		h.world.Step(flaps)
		if h.world.Alive() == 0 || h.world.Tick >= protocol.MaxRaceTicks {
			h.finishRace(now)
		} else {
			h.maybeSnapshot(now)
		}
	case protocol.StFinished:
		if now >= h.finishAt {
			if len(h.players) > 0 {
				h.beginCountdown(now) // everyone connected becomes a racer
				h.broadcast()
			} else {
				h.st = protocol.StWaiting
				h.world, h.raceInfo, h.results = nil, nil, nil
			}
		} else {
			h.maybeSnapshot(now)
		}
	case protocol.StWaiting:
		// idle: no race exists; joins start the countdown
	}
}

// beginCountdown resets the world for a fresh race with a new random theme
// (never the same twice in a row) and admits every connected player. The
// caller broadcasts.
func (h *Hub) beginCountdown(now int64) {
	h.st = protocol.StCountdown
	h.goAt = now + h.cfg.Countdown.Milliseconds()
	h.seed = h.rng.Next()
	h.theme = h.pickTheme()
	h.world = game.NewWorld(h.seed)
	h.raceInfo = map[uint64]raceInfo{}
	h.results = nil
	for _, id := range h.order {
		s := h.players[id]
		h.world.AddBird(id, s.Name, s.Bird.Palette, s.Bird.Accessory)
		h.raceInfo[id] = raceInfo{s.Name, s.Bird}
	}
}

func (h *Hub) pickTheme() protocol.Theme {
	t := protocol.Theme(h.rng.Next() % uint64(protocol.ThemeCount))
	if t == h.lastTheme && protocol.ThemeCount > 1 {
		t = (t + 1) % protocol.ThemeCount
	}
	return t
}

// admitRacer adds a session that joined during the countdown.
func (h *Hub) admitRacer(sess *Session) {
	h.world.AddBird(sess.ID, sess.Name, sess.Bird.Palette, sess.Bird.Accessory)
	h.raceInfo[sess.ID] = raceInfo{sess.Name, sess.Bird}
}

// finishRace ends the current race: rank it, persist the stats, and open the
// results window — unless nobody is left to watch it, in which case the hub
// goes straight back to waiting (the stats still count).
func (h *Hub) finishRace(now int64) {
	h.results = h.world.Results()
	h.lastTheme = h.theme

	entries := make([]RaceEntry, len(h.results))
	for i, r := range h.results {
		info := h.raceInfo[r.ID] // left players keep their stored info
		entries[i] = RaceEntry{Name: r.Name, Score: r.Score, Rank: r.Rank, Bird: info.bird}
	}
	h.store.RecordRace(entries, h.theme, time.UnixMilli(now))

	if len(h.players) == 0 {
		h.st = protocol.StWaiting
		h.world, h.raceInfo, h.results = nil, nil, nil
		return
	}
	h.st = protocol.StFinished
	h.finishAt = now + h.cfg.Results.Milliseconds()
	h.broadcast()
}

// — helpers —

// birdTaken reports whether a bird combination is in use right now, so that
// fresh assignments stay visually distinct while a lobby fills up.
func (h *Hub) birdTaken(b protocol.Bird) bool {
	for _, s := range h.players {
		if s.Bird == b {
			return true
		}
	}
	return false
}

func (h *Hub) uniqueName(base string) string {
	name := base
	for n := 2; h.names[name] != 0; n++ {
		name = fmt.Sprintf("%s %d", base, n)
	}
	return name
}

func (h *Hub) reject(c *clientConn, msg string) {
	c.enqueue(mustMarshal(protocol.ErrorMsg{T: protocol.TError, Msg: msg}))
	c.close()
}

func (h *Hub) sendWelcome(sess *Session, now int64) {
	w := protocol.WelcomeMsg{
		T:     protocol.TWelcome,
		You:   sess.ID,
		Name:  sess.Name,
		Bird:  sess.Bird,
		Now:   now,
		St:    h.st,
		Theme: uint8(h.theme),
		Spect: h.st == protocol.StRacing || h.st == protocol.StFinished,
	}
	if h.st != protocol.StWaiting {
		w.GoAt = h.goAt
		w.Seed = h.seed
	}
	sess.conn.enqueue(mustMarshal(w))
}

// maybeSnapshot keeps the 30 Hz broadcast cadence.
func (h *Hub) maybeSnapshot(now int64) {
	if now-h.lastSnap >= h.snapshotEvery {
		h.broadcast()
	}
}

// broadcast marshals one snapshot and queues the same bytes to everyone.
func (h *Hub) broadcast() {
	if h.st == protocol.StWaiting || len(h.players) == 0 {
		return
	}
	now := h.now()
	h.lastSnap = now
	snap := protocol.Snapshot{
		T:     protocol.TSnapshot,
		Now:   now,
		St:    h.st,
		GoAt:  h.goAt,
		Theme: uint8(h.theme),
		Seed:  h.seed,
		Wait:  h.spectatorCount(),
	}
	if h.world != nil {
		snap.Tick = h.world.Tick
		snap.Dist = h.world.Dist
		snap.Birds = h.world.BirdSnaps()
		snap.Pipes = h.world.PipeSnaps()
	}
	if h.st == protocol.StFinished {
		snap.Res = h.results
	}
	b := mustMarshal(snap)
	for _, id := range h.order {
		if s, ok := h.players[id]; ok {
			s.conn.enqueue(b)
		}
	}
}

func (h *Hub) spectatorCount() int {
	n := 0
	for _, id := range h.order {
		if _, ok := h.players[id]; ok && (h.world == nil || !h.world.IsRacer(id)) {
			n++
		}
	}
	return n
}

// State exposes the current wire state for tests and health reporting.
func (h *Hub) State() uint8 { return h.st }

func mustMarshal(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err) // protocol types are marshal-safe by construction
	}
	return b
}