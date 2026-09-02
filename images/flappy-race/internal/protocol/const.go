// Package protocol is the shared leaf package between server and client.
// It owns the wire format, the bird/appearance vocabulary and every tuning
// constant so that both sides can never drift on game feel.
package protocol

// Logical viewport: the world is CanvasH px tall (every gameplay constant
// lives in that vertical space); the visible width adapts to the screen
// aspect, clamped to ViewMinW..ViewMaxW.
const (
	CanvasW  = 480  // reference width: sky image and the fallback view
	CanvasH  = 800  // world height (ground strip at GroundY)
	ViewMinW = 360  // narrow portrait phones
	ViewMaxW = 3200 // super-ultrawide guard
)

// Simulation constants, all expressed per 60 Hz tick. Derived from the
// classic FlapPy Bird feel (30 FPS in 288×512 space: gravity 1, flap −9,
// terminal 10, scroll 4 px/frame) by scaling velocities ÷2, accelerations
// ÷4 and lengths ×1.5625 (800/512) — then rounded to tidy values.
const (
	TickHz = 60

	Gravity      = 0.42 // px/tick² added to vy each tick
	FlapImpulse  = -7.2 // vy is SET to this on flap (never added)
	TerminalVel  = 8.0  // vy clamp while falling
	BirdX        = 120.0 // fixed x position of every bird
	BirdRadius   = 11.0  // circle collision radius
	PipeWidth    = 80.0
	PipeGap      = 156.0 // gap height; shrinks with the difficulty ramp
	PipeSpacing  = 260.0 // center-to-center distance between pipe pairs
	ScrollSpeed  = 3.1   // px/tick world scroll base speed (≈1.4 s per pipe)
	FirstPipeX   = 440.0 // first pipe reaches the bird ~2.2 s after GO
	GapYMin      = 168.0 // gap center bounds
	GapYMax      = 552.0
	MaxGapDelta  = 240.0 // adjacent gap centers may differ by at most this
	GroundY      = 720.0 // top of the ground strip; birds die below it
	CeilingY     = 11.0  // birds clamp below the ceiling (no death)
	SpawnY       = 400.0 // every bird starts here
	FlapAnimTicks = 12   // a flap animates the wings for this many ticks
)

// Difficulty ramp: bounds race duration. Speed rises, gap shrinks.
const (
	RampEveryTicks = 900          // every 15 s …
	RampSpeedMult  = 1.0625       // … speed ×6.25 %, …
	RampSpeedCap   = 4.65         // … capped at +50 %, …
	RampGapShrink  = 2.0          // … gap −2 px …
	RampGapFloor   = 144.0        // … but never below this.
	MaxRaceTicks   = 120 * TickHz // hard force-finish after 120 s
)

// Snapshot cadence and interpolation tuning.
const (
	SnapshotHz = 30
	SnapshotEvery = 2 // hub ticks per snapshot (60/2 = 30 Hz)
	InterpDelayMs = 90 // render this far behind server time
	SnapHoldMs    = 120 // hold newest snapshot this long before freezing
	SnapGapMs     = 150 // larger gaps between snapshots snap instead of lerp
	RingLen       = 8
)

// Limits.
const (
	MaxPlayers    = 64
	ReadLimit     = 4096 // bytes per inbound frame
	MaxNameLen    = 16   // runes
	FlapRateLimit = 20   // flaps per second per player
	JoinTimeoutMs = 5000 // a connection must send its join within this window
	MaxTopEntries = 50   // persisted all-time leaderboard size
	MaxLeaderboardRows = 8 // shown in-game
)

// Race states (wire values).
const (
	StWaiting   uint8 = 0 // server idle, no race
	StCountdown uint8 = 1
	StRacing    uint8 = 2
	StFinished  uint8 = 3
)