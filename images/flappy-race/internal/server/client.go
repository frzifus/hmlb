package server

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"flappy-race/internal/protocol"
)

// clientConn is the transport half of a session: a gorilla connection with
// a single writer goroutine (gorilla allows exactly one concurrent writer)
// fed by a drop-oldest queue. Snapshots are idempotent, so dropping the
// oldest queued frame when a client stalls is always correct.
//
// Ownership: the hub goroutine is the only enqueuer and the only closer of
// the queue; the read pump merely notifies the hub when the connection
// dies. close() is idempotent because both the join-failure path and the
// leave path may run.
type clientConn struct {
	ws        *websocket.Conn
	send      chan []byte
	closeOnce sync.Once
}

const (
	sendQueueLen = 64
	writeWait    = 10 * time.Second
	pongWait     = 60 * time.Second
	pingPeriod   = 25 * time.Second
)

func newClientConn(ws *websocket.Conn) *clientConn {
	return &clientConn{ws: ws, send: make(chan []byte, sendQueueLen)}
}

// enqueue hands a frame to the writer. Never blocks: a full queue means the
// client is hopelessly behind — drop the oldest frame and keep the newest.
func (c *clientConn) enqueue(b []byte) {
	for {
		select {
		case c.send <- b:
			return
		default:
			select {
			case <-c.send: // drop oldest, retry
			default:
			}
		}
	}
}

// close drains-and-stops the writer exactly once.
func (c *clientConn) close() {
	c.closeOnce.Do(func() { close(c.send) })
}

// serveConn starts the writer and blocks in the reader until the
// connection ends.
func (h *Hub) serveConn(c *clientConn) {
	go h.writePump(c)
	h.readPump(c)
}

// clientFrame is the tagged union of every inbound frame type.
type clientFrame struct {
	T    uint8  `json:"t"`
	Name string `json:"name"`
}

// readFrame reads one complete message and decodes it. Garbage frames
// decode to the zero value (unknown types are ignored by callers) instead
// of killing the connection; only transport errors return false.
func readFrame(ws *websocket.Conn, v *clientFrame) bool {
	_, data, err := ws.ReadMessage()
	if err != nil {
		return false
	}
	if json.Unmarshal(data, v) != nil {
		*v = clientFrame{}
	}
	return true
}

// readPump consumes inbound frames. The first frame must be the join; later
// frames may only be flaps. The hub owns all cleanup.
func (h *Hub) readPump(c *clientConn) {
	var sess *Session
	defer func() { h.leave(c, sess) }()

	c.ws.SetReadLimit(protocol.ReadLimit)
	_ = c.ws.SetReadDeadline(time.Now().Add(protocol.JoinTimeoutMs * time.Millisecond))

	var f clientFrame
	if !readFrame(c.ws, &f) || f.T != protocol.TJoin {
		return // no (valid) join in time: drop the connection silently
	}
	sess, errMsg := h.join(c, f.Name)
	if errMsg != "" || sess == nil {
		return // hub has queued the error and closed the queue
	}

	_ = c.ws.SetReadDeadline(time.Now().Add(pongWait))
	c.ws.SetPongHandler(func(string) error {
		return c.ws.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		f = clientFrame{}
		if !readFrame(c.ws, &f) {
			return
		}
		if f.T == protocol.TFlap {
			h.flap(sess)
		}
		_ = c.ws.SetReadDeadline(time.Now().Add(pongWait))
	}
}

// writePump owns every write on the connection.
func (h *Hub) writePump(c *clientConn) {
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()
	defer c.ws.Close()
	for {
		select {
		case msg, ok := <-c.send:
			_ = c.ws.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.ws.WriteMessage(websocket.CloseMessage,
					websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
				return
			}
			if err := c.ws.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.ws.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.ws.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

