package client

import (
	"context"
	"encoding/json"
	"time"

	"github.com/coder/websocket"

	"flappy-race/internal/protocol"
)

// DefaultSess is the process-wide session identity the reader fills in on
// the welcome frame (the wasm entry point reads it after connecting).
var DefaultSess Sess

// Net owns the WebSocket connection. One goroutine owns all I/O: reads are
// dispatched (snapshots go onto a drop-oldest channel that Update drains,
// session facts go through Sess), writes are fed by a small command queue
// fed by Flap(). No render-loop call ever blocks on the network.
type Net struct {
	ws     *websocket.Conn
	cmdCh  chan []byte
	snapCh chan protocol.Snapshot
	errCh  chan error
	closed chan struct{}
}

func Dial(ctx context.Context, url string) (*Net, error) {
	ws, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		return nil, err
	}
	return &Net{
		ws:     ws,
		cmdCh:  make(chan []byte, 8),
		snapCh: make(chan protocol.Snapshot, protocol.RingLen),
		errCh:  make(chan error, 1),
		closed: make(chan struct{}),
	}, nil
}

// Join sends the mandatory first frame and starts the reader/writer.
func (n *Net) Join(name string) error {
	b, err := json.Marshal(protocol.JoinMsg{T: protocol.TJoin, Name: name})
	if err != nil {
		return err
	}
	wctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := n.ws.Write(wctx, websocket.MessageText, b); err != nil {
		return err
	}
	go n.writeLoop()
	go n.readLoop()
	return nil
}

// Close drops a connection whose join failed — its reader/writer loops
// never started, so this is the only cleanup it needs (fail() is for
// connections that are already running).
func (n *Net) Close() {
	_ = n.ws.Close(websocket.StatusNormalClosure, "bye")
}

// Flap queues an input impulse; never blocks the render loop.
func (n *Net) Flap() {
	select {
	case n.cmdCh <- []byte(`{"t":2}`):
	default: // queue full: coalesced flaps are harmless, drop it
	}
}

// SnapCh yields parsed snapshots (the receiver drains and keeps the newest).
func (n *Net) SnapCh() <-chan protocol.Snapshot { return n.snapCh }

// ErrCh delivers the first fatal network error.
func (n *Net) ErrCh() <-chan error { return n.errCh }

// Closed reports when the reader loop has stopped.
func (n *Net) Closed() <-chan struct{} { return n.closed }

func (n *Net) writeLoop() {
	for {
		select {
		case msg := <-n.cmdCh:
			wctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			if err := n.ws.Write(wctx, websocket.MessageText, msg); err != nil {
				cancel()
				n.fail(err)
				return
			}
			cancel()
		case <-n.closed:
			return
		}
	}
}

func (n *Net) readLoop() {
	defer close(n.closed)
	for {
		_, data, err := n.ws.Read(context.Background())
		if err != nil {
			n.fail(err)
			return
		}
		var env protocol.Envelope
		if json.Unmarshal(data, &env) != nil {
			continue
		}
		switch env.T {
		case protocol.TSnapshot:
			var s protocol.Snapshot
			if json.Unmarshal(data, &s) == nil {
				n.pushSnap(s)
			}
		case protocol.TWelcome:
			var w protocol.WelcomeMsg
			if json.Unmarshal(data, &w) == nil {
				DefaultSess.Set(w.You, w.Name, w.Bird.Palette, w.Bird.Accessory, w.Spect)
			}
		case protocol.TError:
			var e protocol.ErrorMsg
			if json.Unmarshal(data, &e) == nil {
				n.fail(errFatal{msg: e.Msg})
			}
		}
	}
}

// pushSnap delivers the newest snapshot, dropping the stale one when the
// render loop falls behind (snapshots are idempotent).
func (n *Net) pushSnap(s protocol.Snapshot) {
	select {
	case n.snapCh <- s:
		return
	default:
	}
	select {
	case <-n.snapCh:
	default:
	}
	select {
	case n.snapCh <- s:
	default:
	}
}

func (n *Net) fail(err error) {
	select {
	case n.errCh <- err:
	default:
	}
	_ = n.ws.Close(websocket.StatusNormalClosure, "bye")
}

// errFatal marks server- or connection-originated failures.
type errFatal struct{ msg string }

func (e errFatal) Error() string { return e.msg }