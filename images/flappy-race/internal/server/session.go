package server

import "flappy-race/internal/protocol"

// Session is one connected player. It is owned by the hub goroutine; the
// transport goroutines only read the immutable fields (ID, Name, Bird).
type Session struct {
	ID   uint64
	Name string
	Bird protocol.Bird

	conn *clientConn

	// flap rate limiting (hub goroutine only)
	flapSec   int64
	flapCount int
}