package protocol

import "time"

// Snapshot is the complete authoritative world state at a point in time,
// broadcast ~30 times per second. It is deliberately idempotent: a client
// can rebuild its entire view from any single snapshot. Birds carry their
// names and appearance inline so that no separate roster bookkeeping is
// needed (dead or disconnected players would otherwise become nameless).
type Snapshot struct {
	T     uint8        `json:"t"` // always TSnapshot
	Now   int64        `json:"now"`  // server wall-clock ms — the time source
	Tick  uint64       `json:"k"`    // simulation tick since race start
	St    uint8        `json:"st"`
	GoAt  int64        `json:"go,omitempty"` // countdown target / race start, ms
	Theme uint8        `json:"th"`
	Seed  uint64       `json:"sd,omitempty"` // per-race seed for procedural scenery
	Dist  float64      `json:"ds"`            // world distance scrolled (parallax)
	Wait  int          `json:"w,omitempty"`  // connected players waiting for the next race
	Birds []BirdSnap   `json:"b,omitempty"`
	Pipes []PipeSnap   `json:"p,omitempty"` // visible + near-offscreen, ordered x ASC
	Res   []ResultSnap `json:"r,omitempty"` // only while St == StFinished
}

// BirdSnap is one racer's state. Dead birds stay in the list until the race
// ends so that everyone can watch the remaining scores climb.
type BirdSnap struct {
	ID    uint64  `json:"i"`
	Name  string  `json:"n"`
	Y     float64 `json:"y"`
	Vy    float64 `json:"v"`
	Score int     `json:"s"`
	Dead  bool    `json:"d"`
	Left  bool    `json:"l"` // left the server mid-race
	Flap  bool    `json:"f"` // flapped within the last FlapAnimTicks ticks
	Pal   uint8   `json:"c"` // palette
	Acc   uint8   `json:"k"` // accessory
}

// PipeSnap is one pipe pair; gap center G with total gap height H.
// IDs are stable for the lifetime of a race, which is what lets clients
// interpolate positions between snapshots.
type PipeSnap struct {
	ID uint64  `json:"i"`
	X  float64 `json:"x"`
	G  float64 `json:"g"`
	H  float64 `json:"h"`
}

// ResultSnap is one podium entry; names are inline so that players who left
// mid-race still show up correctly.
type ResultSnap struct {
	ID    uint64 `json:"i"`
	Name  string `json:"n"`
	Score int    `json:"s"`
	Rank  int    `json:"r"`
	Left  bool   `json:"l"`
}

// TopEntry is one row of the persistent all-time leaderboard, served by
// /api/leaderboard and shown on the start screen.
type TopEntry struct {
	Name  string    `json:"name"`
	Score int       `json:"score"`
	Theme string    `json:"theme,omitempty"`
	At    time.Time `json:"at"`
}