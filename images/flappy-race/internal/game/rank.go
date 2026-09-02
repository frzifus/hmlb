package game

import (
	"sort"

	"flappy-race/internal/protocol"
)

// Results ranks a finished race into a deterministic total order:
// score DESC, then later death first (surviving longer is better), then id
// ASC — results never wobble between snapshots. Birds still alive (the race
// was force-finished at MaxRaceTicks) count as having died at the cap.
func (w *World) Results() []protocol.ResultSnap {
	type row struct {
		id, death uint64
		name      string
		score     int
		left      bool
	}
	rows := make([]row, 0, len(w.Order))
	for _, id := range w.Order {
		b := w.Birds[id]
		death := b.DeathTick
		if !b.Dead {
			death = protocol.MaxRaceTicks
		}
		rows = append(rows, row{b.ID, death, b.Name, b.Score, b.Left})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].score != rows[j].score {
			return rows[i].score > rows[j].score
		}
		if rows[i].death != rows[j].death {
			return rows[i].death > rows[j].death
		}
		return rows[i].id < rows[j].id
	})
	out := make([]protocol.ResultSnap, len(rows))
	for i, r := range rows {
		out[i] = protocol.ResultSnap{
			ID:    r.id,
			Name:  r.name,
			Score: r.score,
			Rank:  i + 1,
			Left:  r.left,
		}
	}
	return out
}