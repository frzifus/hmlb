package server

import "time"

// Config is the operator-facing server configuration. Durations are
// injectable so tests can fast-forward the race lifecycle; the simulation
// itself is always real 60 Hz physics.
type Config struct {
	Addr        string // listen address, e.g. ":8080"
	DataPath    string // JSON state file for the leaderboard
	Countdown   time.Duration
	Results     time.Duration
	SnapshotHz  int // 0 = protocol default
	MaxPlayers  int // 0 = protocol default
}

func DefaultConfig() Config {
	return Config{
		Addr:      ":8080",
		DataPath:  "data/state.json",
		Countdown: 3 * time.Second,
		Results:   6 * time.Second,
	}
}