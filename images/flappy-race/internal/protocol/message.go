package protocol

import "unicode"

// Message types on the wire. Frequent state travels exclusively through
// idempotent snapshots; rare session facts travel as one-off messages.
const (
	// client → server
	TJoin uint8 = 1
	TFlap uint8 = 2

	// server → client
	TWelcome  uint8 = 10
	TSnapshot uint8 = 11
	TError    uint8 = 13
)

// JoinMsg is the mandatory first frame a client sends after connecting.
type JoinMsg struct {
	T    uint8  `json:"t"`
	Name string `json:"name"`
}

// FlapMsg requests an upward impulse at the next simulation tick.
type FlapMsg struct {
	T uint8 `json:"t"`
}

// WelcomeMsg is the first frame a client receives after a successful join.
type WelcomeMsg struct {
	T     uint8  `json:"t"`
	You   uint64 `json:"you"`
	Name  string `json:"name"`
	Bird  Bird   `json:"bird"`
	Now   int64  `json:"now"`
	St    uint8  `json:"st"`
	GoAt  int64  `json:"go,omitempty"` // countdown target / race start, ms
	Theme uint8  `json:"th"`
	Seed  uint64 `json:"sd,omitempty"`
	Spect bool   `json:"spect"` // joined mid-race: waiting for the next one
}

// ErrorMsg is a fatal server message; the connection is closed right after.
type ErrorMsg struct {
	T   uint8  `json:"t"`
	Msg string `json:"msg"`
}

// Envelope is the tagged union every frame unmarshals into first.
type Envelope struct {
	T uint8 `json:"t"`
}

// ValidateName trims and checks a username: 1..MaxNameLen printable runes.
// The cleaned name and whether it is acceptable are returned.
func ValidateName(s string) (string, bool) {
	name := []rune(s)
	// trim leading/trailing whitespace
	for len(name) > 0 && unicode.IsSpace(name[0]) {
		name = name[1:]
	}
	for len(name) > 0 && unicode.IsSpace(name[len(name)-1]) {
		name = name[:len(name)-1]
	}
	if len(name) == 0 || len(name) > MaxNameLen {
		return string(name), false
	}
	for _, r := range name {
		if !unicode.IsPrint(r) {
			return string(name), false
		}
	}
	return string(name), true
}