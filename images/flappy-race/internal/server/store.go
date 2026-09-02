package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"sync/atomic"
	"time"

	"flappy-race/internal/protocol"
)

// Store persists the all-time leaderboard and per-player state to a JSON
// file. The hub goroutine is the only writer (RecordRace / RememberBird);
// HTTP readers get an immutable byte snapshot via APIBytes, published
// atomically after every change. Writes go through a temp file + rename so
// a crash never corrupts the previous state.
type Store struct {
	path string
	log  *slog.Logger
	db   stateDB
	api  atomic.Value // []byte, refreshed after every mutation
}

type stateDB struct {
	Updated time.Time              `json:"updated"`
	Top     []protocol.TopEntry    `json:"top"`
	Players map[string]PlayerStat  `json:"players"`
}

// PlayerStat is everything remembered about a username across sessions.
type PlayerStat struct {
	Best      int            `json:"best"`
	BestTheme string         `json:"best_theme"`
	BestAt    time.Time      `json:"best_at"`
	Races     int            `json:"races"`
	Wins      int            `json:"wins"`
	Bird      protocol.Bird  `json:"bird"`
}

// OpenStore loads (or creates) the state file. A corrupt or unreadable file
// is a warning, not a fatal error: the store starts fresh rather than taking
// the server down. The error is returned for the caller to log.
func OpenStore(path string) (*Store, error) {
	s := &Store{path: path, log: slog.Default(), db: stateDB{Players: map[string]PlayerStat{}}}
	data, err := os.ReadFile(path)
	switch {
	case errors.Is(err, os.ErrNotExist): // fresh store
	case err != nil:
		s.refreshAPI()
		return s, err
	default:
		if err := json.Unmarshal(data, &s.db); err != nil {
			s.db = stateDB{Players: map[string]PlayerStat{}}
			s.refreshAPI()
			return s, fmt.Errorf("state file %s corrupt, starting fresh: %w", path, err)
		}
		if s.db.Players == nil {
			s.db.Players = map[string]PlayerStat{}
		}
	}
	s.refreshAPI()
	return s, nil
}

// APIBytes returns the serialized all-time leaderboard served at
// /api/leaderboard. The bytes are owned by the caller.
func (s *Store) APIBytes() []byte {
	b, _ := s.api.Load().([]byte)
	return b
}

// LookupBird returns the bird remembered for a username, if any.
func (s *Store) LookupBird(name string) (protocol.Bird, bool) {
	ps, ok := s.db.Players[name]
	return ps.Bird, ok && ps.Bird.Valid()
}

// RememberBird stores the bird assigned to a new username.
func (s *Store) RememberBird(name string, bird protocol.Bird) {
	ps := s.db.Players[name] // zero value if the player raced before join… fine
	ps.Bird = bird
	s.db.Players[name] = ps
	s.persist("remember bird")
}

// RaceEntry is one finisher handed to RecordRace.
type RaceEntry struct {
	Name  string
	Score int
	Rank  int
	Bird  protocol.Bird
}

// RecordRace folds a finished race into the persistent stats: races/wins/
// best per player, and the all-time top list (best run per username).
func (s *Store) RecordRace(entries []RaceEntry, theme protocol.Theme, at time.Time) {
	for _, e := range entries {
		ps := s.db.Players[e.Name]
		ps.Bird = e.Bird
		ps.Races++
		if e.Rank == 1 {
			ps.Wins++
		}
		if e.Score > ps.Best {
			ps.Best = e.Score
			ps.BestTheme = theme.String()
			ps.BestAt = at
		}
		s.db.Players[e.Name] = ps
	}
	s.rebuildTop()
	s.db.Updated = at
	s.persist("record race")
}

// rebuildTop derives the all-time list (best run per username, score desc,
// name asc) from the player stats.
func (s *Store) rebuildTop() {
	top := make([]protocol.TopEntry, 0, len(s.db.Players))
	for name, ps := range s.db.Players {
		if ps.Best <= 0 {
			continue
		}
		top = append(top, protocol.TopEntry{
			Name:  name,
			Score: ps.Best,
			Theme: ps.BestTheme,
			At:    ps.BestAt,
		})
	}
	sort.Slice(top, func(i, j int) bool {
		if top[i].Score != top[j].Score {
			return top[i].Score > top[j].Score
		}
		return top[i].Name < top[j].Name
	})
	if len(top) > protocol.MaxTopEntries {
		top = top[:protocol.MaxTopEntries]
	}
	s.db.Top = top
}

// persist saves the state atomically and refreshes the API bytes. A failed
// save is logged but never fatal — losing history is better than dying.
func (s *Store) persist(context string) {
	if err := s.save(); err != nil {
		s.log.Error("store save failed", "err", err, "path", s.path, "op", context)
	}
	s.refreshAPI()
}

func (s *Store) save() error {
	data, err := json.MarshalIndent(s.db, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func (s *Store) refreshAPI() {
	payload := struct {
		Updated time.Time           `json:"updated"`
		Top     []protocol.TopEntry `json:"top"`
	}{s.db.Updated, s.db.Top}
	b, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		b = []byte("{}")
	}
	s.api.Store(b)
}