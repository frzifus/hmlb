package client

import (
	"sort"

	"flappy-race/internal/protocol"
)

// HUDRow is one live leaderboard row.
type HUDRow struct {
	ID    uint64
	Name  string
	Score int
	Alive bool
	Left  bool
	Self  bool
	Rank  int
}

// HUD is everything the overlay needs for one frame, derived from the
// newest authoritative snapshot (never interpolated).
type HUD struct {
	Countdown string // "3" / "2" / "1" / "" ; the GO flash is the Go flag
	Go        bool
	OwnScore  int
	Rows      []HUDRow
	Waiting   int
	Dead      bool // own bird died; spectating the rest
	Spect     bool // joined mid-race, waiting for the next one
	Results   []protocol.ResultSnap
}

// DeriveHUD builds the HUD state for one frame.
func DeriveHUD(snap protocol.Snapshot, serverNow int64, selfID uint64) HUD {
	h := HUD{
		Results: snap.Res,
		Waiting: snap.Wait,
	}

	switch snap.St {
	case protocol.StCountdown:
		if remaining := snap.GoAt - serverNow; remaining > 0 {
			h.Countdown = itoa(clampInt(int((remaining+999)/1000), 1, 3))
		} else {
			h.Go = true // the local number hit zero: flash until the flip
		}
	case protocol.StRacing:
		// "LET'S GO!" lingers briefly after the flip.
		if serverNow-snap.GoAt < 700 {
			h.Go = true
		}
	}

	// A player is spectating when they are connected but fly no bird.
	h.Spect = true
	for _, b := range snap.Birds {
		if b.ID == selfID {
			h.OwnScore = b.Score
			h.Dead = b.Dead
			h.Spect = false
		}
	}

	// Live leaderboard: alive racers by score first, then the fallen.
	rows := make([]HUDRow, len(snap.Birds))
	for i, b := range snap.Birds {
		rows[i] = HUDRow{
			ID:    b.ID,
			Name:  b.Name,
			Score: b.Score,
			Alive: !b.Dead,
			Left:  b.Left,
			Self:  b.ID == selfID,
		}
	}
	sort.SliceStable(rows, func(i, j int) bool {
		ai, aj := rows[i].Alive, rows[j].Alive
		if ai != aj {
			return ai
		}
		if rows[i].Score != rows[j].Score {
			return rows[i].Score > rows[j].Score
		}
		return rows[i].ID < rows[j].ID
	})
	for i := range rows {
		rows[i].Rank = i + 1
	}
	if len(rows) > protocol.MaxLeaderboardRows {
		rows = rows[:protocol.MaxLeaderboardRows]
	}
	h.Rows = rows
	return h
}

func itoa(n int) string {
	digits := "0123456789"
	if n >= 0 && n <= 9 {
		return digits[n : n+1]
	}
	s := ""
	if n < 0 {
		s = "-"
		n = -n
	}
	var buf [8]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = digits[n%10]
		n /= 10
	}
	return s + string(buf[i:])
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}