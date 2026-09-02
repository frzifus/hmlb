package client

import (
	"strings"

	"flappy-race/internal/protocol"
)

// applyNameInput folds one frame of start-screen input into the buffer:
// typed characters append (capped at MaxNameLen), backspace drops the last
// rune, and Enter submits — through protocol.ValidateName, so the screen
// and the server enforce exactly the same rule. A submit returns the
// cleaned name and clears the buffer; anything else returns "".
func applyNameInput(buf []rune, chars []rune, backspace, enter bool) ([]rune, string) {
	for _, r := range chars {
		if len(buf) >= protocol.MaxNameLen {
			break
		}
		buf = append(buf, r)
	}
	if backspace && len(buf) > 0 {
		buf = buf[:len(buf)-1]
	}
	if enter {
		if name, ok := protocol.ValidateName(string(buf)); ok {
			return buf[:0], name
		}
	}
	return buf, ""
}

// httpBase turns the WebSocket URL into the site root for plain HTTP
// side calls (the start screen's leaderboard fetch): ws→http, wss→https,
// and the /ws path dropped.
func httpBase(wsURL string) string {
	base := strings.TrimSuffix(wsURL, "/ws")
	switch {
	case strings.HasPrefix(base, "wss://"):
		return "https://" + strings.TrimPrefix(base, "wss://")
	case strings.HasPrefix(base, "ws://"):
		return "http://" + strings.TrimPrefix(base, "ws://")
	}
	return base
}