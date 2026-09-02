//go:build js && wasm || native

package client

import (
	"context"
	"encoding/json"
	"fmt"
	"image/color"
	"math"
	"net/http"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"flappy-race/internal/client/draw"
	"flappy-race/internal/protocol"
)

const otherBirdAlpha = 0.55 // requirement 4: other birds are translucent

var (
	hudNeutralColor  = color.RGBA{255, 255, 255, 255}
	hudErrorColor    = color.RGBA{255, 120, 110, 255}
	hudErrorSubColor = color.RGBA{255, 190, 185, 220}
	entrySubColor    = color.RGBA{159, 176, 216, 255} // the web start card's muted blue-gray
)

// Game is the Ebitengine game: a dumb renderer over the authoritative
// server state plus a flap-input sender. The connection is (re)made from
// Update: a server that is down — at load time or after a restart — is
// retried with backoff instead of leaving the player on a dead screen.
type Game struct {
	net  *Net
	clk  *Clock
	ring *Ring

	sprites *draw.Sprites
	text    *draw.Texter
	scenery *draw.Scenery
	themeID uint8
	seed    uint64

	url  string // server WebSocket URL, fixed for the session
	name string // join name; the server may dedupe it (the welcome says so)

	viewW int // logical view width; adapts to the screen in Layout

	frame   uint64
	fatal   string // server-sent rejection (bad name); retrying cannot fix it
	lastErr string // most recent dial/join/connection failure, for the overlay

	// Start screen, the in-window twin of the web's: shown when no name
	// was handed in; the all-time top 10 loads while the player types.
	entering    bool
	typed       []rune
	entryErr    bool
	boardCh     chan boardRes
	board       []protocol.TopEntry
	boardLoaded bool

	dialing       bool         // a dial goroutine is in flight
	dialRes       chan dialRes // its result; buffered 1, drained by Update
	attempts      int          // consecutive failed attempts
	nextTry       time.Time    // when the next attempt is due (zero: right away)
	everConnected bool         // has any join succeeded yet (overlay copy)
}

// NewGame prepares the game for url/name. Without a name the game opens
// on its start screen — the same one the web serves: type a name, see the
// all-time top 10, Enter joins. Connecting itself happens from Update
// (and keeps happening while it fails), so a player starting before the
// server is up sees a retrying screen, never a dead one.
func NewGame(url, name string) *Game {
	g := &Game{
		clk:     NewClock(func() int64 { return time.Now().UnixMilli() }),
		ring:    NewRing(),
		sprites: draw.NewSprites(),
		viewW:   protocol.CanvasW,
		url:     url,
		name:    name,
		dialRes: make(chan dialRes, 1),
		boardCh: make(chan boardRes, 1),
	}
	if t, err := draw.NewText(); err == nil {
		g.text = t
	}
	if name == "" {
		g.entering = true
		go g.fetchBoard(httpBase(url))
	}
	return g
}

// boardRes delivers the start screen's leaderboard; best-effort: nothing
// is delivered when the fetch fails and the board section is skipped.
type boardRes struct{ top []protocol.TopEntry }

// fetchBoard loads the all-time top 10 from the server behind the given
// WebSocket URL (mirroring the web start screen's fetch).
func (g *Game) fetchBoard(base string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/api/leaderboard", nil)
	if err != nil {
		return
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	var lb struct {
		Top []protocol.TopEntry `json:"top"`
	}
	if json.NewDecoder(resp.Body).Decode(&lb) != nil {
		return
	}
	select {
	case g.boardCh <- boardRes{top: lb.Top}:
	default:
	}
}

// Update drains the network, drives the reconnect state machine and
// forwards input at 60 TPS.
func (g *Game) Update() error {
	g.frame++
	if g.entering {
		g.enterName()
		return nil
	}
	if g.fatal != "" {
		return nil // permanent state; nothing further can happen
	}
	if g.net != nil {
		g.pumpNet()
	}
	if g.net == nil && g.fatal == "" {
		g.reconnect()
	}
	return nil
}

// enterName reads one frame of start-screen input: typed characters,
// backspace, and Enter to join. The buffer rules live in applyNameInput
// (host-tested); this is only the ebiten plumbing around them.
func (g *Game) enterName() {
	select {
	case res := <-g.boardCh:
		g.board = res.top
		g.boardLoaded = true
	default:
	}
	chars := ebiten.AppendInputChars(nil)
	backspace := inpututil.IsKeyJustPressed(ebiten.KeyBackspace)
	enter := inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeyNumpadEnter)
	if len(chars) > 0 || backspace {
		g.entryErr = false // typing again clears the last rejection
	}
	var submitted string
	g.typed, submitted = applyNameInput(g.typed, chars, backspace, enter)
	if submitted == "" {
		if enter {
			g.entryErr = true // the web start screen's "pick a name" case
		}
		return
	}
	g.name = submitted
	g.entering = false // the next Update starts connecting
}

// pumpNet folds fresh snapshots into the clock and ring, forwards flap
// input, and moves a broken connection into the reconnect state.
func (g *Game) pumpNet() {
	for {
		select {
		case s := <-g.net.SnapCh():
			g.clk.Sample(s.Now)
			g.ring.Push(s)
			continue
		default:
		}
		break
	}
	select {
	case err := <-g.net.ErrCh():
		g.net = nil
		if ef, ok := err.(errFatal); ok {
			g.fatal = ef.msg // the server rejected us (bad name); no retry helps
			return
		}
		g.lastErr = "connection lost"
		g.scheduleRetry()
		return
	default:
	}
	if flapPressed() {
		g.net.Flap()
	}
}

// dialRes is the outcome of one connection attempt; net is set only when
// dial and join both succeeded (its reader/writer loops already run).
type dialRes struct {
	net *Net
	err error
}

// reconnect re-establishes the connection when there is none. Attempts run
// on a goroutine — a dial against a wedged server must not stall the
// render loop — and space out with capped backoff. The join is part of
// the attempt: a connection that cannot carry the join frame is worth as
// little as none. Rejoining prefers the server-deduped name so the same
// bird (persisted per username) comes back after a server restart.
func (g *Game) reconnect() {
	if !g.dialing {
		if !g.nextTry.IsZero() && time.Now().Before(g.nextTry) {
			return
		}
		g.dialing = true
		url := g.url
		name := DefaultSess.Name()
		if name == "" {
			name = g.name // never welcomed yet: the first connection
		}
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			net, err := Dial(ctx, url)
			if err == nil {
				if jerr := net.Join(name); jerr != nil {
					net.Close() // loops never started; drop the socket
					net, err = nil, jerr
				}
			}
			g.dialRes <- dialRes{net: net, err: err}
		}()
		return
	}
	select {
	case res := <-g.dialRes:
		g.dialing = false
		if res.err != nil {
			g.lastErr = res.err.Error()
			g.scheduleRetry()
			return
		}
		g.net = res.net
		g.attempts = 0
		g.lastErr = ""
		g.everConnected = true
		g.ring.Reset() // fresh epoch: never interpolate across the gap
	default:
	}
}

// scheduleRetry records a failed attempt and spaces out the next one.
func (g *Game) scheduleRetry() {
	g.attempts++
	g.nextTry = time.Now().Add(retryDelay(g.attempts))
}

// Draw renders one frame.
func (g *Game) Draw(screen *ebiten.Image) {
	if g.text == nil {
		return // font failed to load; nothing readable can be drawn
	}
	if g.entering {
		g.drawEntry(screen)
		return
	}
	if g.fatal != "" {
		g.drawFrozen(screen, "Disconnected", g.fatal, hudErrorColor, hudErrorSubColor)
		return
	}
	if g.net == nil {
		title, sub := g.reconnStatus()
		g.drawFrozen(screen, title, sub, hudNeutralColor, hudNeutralColor)
		return
	}
	newest, ok := g.ring.Newest()
	if !ok {
		name := DefaultSess.Name()
		if name == "" {
			name = "…"
		}
		g.text.DrawShadowed(screen, "Connecting", 28, float64(g.viewW)/2, 380, hudNeutralColor, true)
		g.text.Draw(screen, "flapping as "+name, 15, float64(g.viewW)/2, 420, hudNeutralColor, true)
		return
	}
	g.drawWorld(screen, newest)
}

// drawFrozen paints the last received world with a status text on top —
// the look of the game while it waits for the server to come back (or,
// after a terminal rejection, forever). Without a world yet, the status
// sits on the dark backdrop alone.
func (g *Game) drawFrozen(dst *ebiten.Image, title, sub string, titleClr, subClr color.Color) {
	if newest, ok := g.ring.Newest(); ok {
		g.drawWorld(dst, newest)
	} else {
		dst.Fill(color.RGBA{11, 16, 32, 255})
	}
	g.text.DrawShadowed(dst, title, 30, float64(g.viewW)/2, 360, titleClr, true)
	g.text.Draw(dst, sub, 14, float64(g.viewW)/2, 404, subClr, true)
}

// reconnStatus is the overlay copy while no connection is live: plain
// "Connecting" before the first join, "Reconnecting" after a drop, with
// the failure reason once attempts have failed.
func (g *Game) reconnStatus() (title, sub string) {
	if !g.everConnected {
		return "Connecting", "waiting for the server…"
	}
	if g.attempts == 0 {
		return "Reconnecting", "connection lost — retrying…"
	}
	return "Reconnecting", fmt.Sprintf("attempt %d — %s", g.attempts, g.lastErr)
}

// drawEntry is the start screen, the in-window twin of the web's card:
// title, tagline, a name line with a blinking caret, and the all-time
// top 10 once it loads (copy matches internal/web/static/index.html).
func (g *Game) drawEntry(dst *ebiten.Image) {
	dst.Fill(color.RGBA{11, 16, 32, 255})
	cx := float64(g.viewW) / 2

	g.text.DrawShadowed(dst, "Flappy Race", 30, cx, 262, hudNeutralColor, true)
	g.text.Draw(dst, "space, click or tap to flap — every pipe counts", 14, cx, 298, entrySubColor, true)

	g.text.Draw(dst, "Your name (shown on the leaderboard)", 12, cx, 356, entrySubColor, true)
	line, clr := string(g.typed), hudNeutralColor
	if line == "" {
		line, clr = "e.g. Swift Falcon", entrySubColor // placeholder
	} else if g.frame/30%2 == 0 {
		line += "_" // blinking caret, half a second on and off
	}
	g.text.Draw(dst, line, 18, cx, 388, clr, true)
	if g.entryErr {
		g.text.Draw(dst, "pick a name (1–16 characters)", 13, cx, 418, hudErrorColor, true)
	}

	if !g.boardLoaded {
		return
	}
	g.text.Draw(dst, "ALL-TIME TOP 10", 12, cx, 466, entrySubColor, true)
	if len(g.board) == 0 {
		g.text.Draw(dst, "no races yet — be the first!", 13, cx, 492, entrySubColor, true)
		return
	}
	for i, e := range g.board {
		if i >= 10 {
			break
		}
		g.text.Draw(dst, fmt.Sprintf("%d. %s — %d", i+1, e.Name, e.Score), 13, cx, float64(492+22*i), hudNeutralColor, true)
	}
}

// drawWorld paints the scene: sky, far, mid, pipes, birds, ground, HUD.
func (g *Game) drawWorld(screen *ebiten.Image, newest protocol.Snapshot) {
	serverNow := g.clk.ServerNow()
	view, ok := g.ring.Interp(serverNow - protocol.InterpDelayMs)
	if !ok {
		return
	}
	g.ensureScenery(view.Theme, view.Seed)
	selfID, _, _ := DefaultSess.Get()

	skyOpts := &ebiten.DrawImageOptions{}
	skyOpts.GeoM.Scale(float64(g.viewW)/float64(protocol.CanvasW), 1) // per-row gradient: horizontal stretch is exact
	screen.DrawImage(g.scenery.Sky, skyOpts)
	g.drawTiled(screen, g.scenery.Far, view.Dist*0.15, 0)
	g.drawTiled(screen, g.scenery.Mid, view.Dist*0.35, 0)

	for _, p := range view.Pipes {
		g.scenery.DrawPipe(screen, float32(p.X), float32(p.GapY), float32(p.GapH))
	}

	// Others first (translucent), the own bird last and opaque.
	for _, b := range view.Birds {
		if b.ID == selfID && !b.Dead {
			continue
		}
		g.drawBird(screen, b, otherBirdAlpha)
	}
	for _, b := range view.Birds {
		if b.ID == selfID && !b.Dead {
			g.drawBird(screen, b, 1)
		}
	}

	g.drawTiled(screen, g.scenery.Ground, view.Dist, protocol.GroundY)

	hud := DeriveHUD(newest, serverNow, selfID)
	draw.DrawHUD(screen, g.text, g.scenery, toDrawHUD(hud, view), float64(g.viewW))
}

// ensureScenery rebuilds the procedural layers when the race theme changes.
func (g *Game) ensureScenery(theme uint8, seed uint64) {
	if g.scenery != nil && g.themeID == theme && g.seed == seed {
		return
	}
	g.scenery = draw.BuildScenery(protocol.Theme(theme), seed)
	g.themeID = theme
	g.seed = seed
}

// drawTiled draws a wrap-around layer shifted by the scroll offset at the
// given y, tiled until it covers the full view width.
func (g *Game) drawTiled(dst *ebiten.Image, layer *ebiten.Image, dist, y float64) {
	w := float64(layer.Bounds().Dx())
	off := w - float64(int(dist)%int(w))
	for x := -off; x < float64(g.viewW); x += w {
		opts := &ebiten.DrawImageOptions{}
		opts.GeoM.Translate(x, y)
		dst.DrawImage(layer, opts)
	}
}

// drawBird renders one bird with the classic tilt and a name label.
func (g *Game) drawBird(dst *ebiten.Image, b BirdView, alpha float32) {
	var frame uint8 = 1
	if b.Flap {
		frame = uint8((g.frame / 5) % 3)
	}
	img := g.sprites.Get(b.Pal, b.Acc, frame)
	opts := &ebiten.DrawImageOptions{}
	opts.Filter = ebiten.FilterLinear
	opts.GeoM.Translate(-draw.SpriteHalfW, -draw.SpriteHalfH)
	opts.GeoM.Rotate(draw.BirdRotation(b.Vy))
	opts.GeoM.Translate(protocol.BirdX, b.Y)
	if alpha != 1 {
		opts.ColorScale.Scale(1, 1, 1, alpha)
	}
	dst.DrawImage(img, opts)

	clr := hudNeutralColor
	if b.Dead {
		clr = hudErrorSubColor
	}
	g.text.DrawShadowed(dst, b.Name, 12, protocol.BirdX, b.Y-draw.NameOffset, clr, true)
}

func toDrawHUD(h HUD, view View) draw.HUDState {
	out := draw.HUDState{
		St:        view.St,
		Countdown: h.Countdown,
		Go:        h.Go,
		OwnScore:  h.OwnScore,
		Wait:      h.Waiting,
		Dead:      h.Dead,
		Spect:     h.Spect,
	}
	for _, r := range h.Rows {
		out.Rows = append(out.Rows, draw.HUDRow{
			Rank: r.Rank, Name: r.Name, Score: r.Score,
			Alive: r.Alive, Left: r.Left, Self: r.Self,
		})
	}
	for _, r := range h.Results {
		out.Results = append(out.Results, draw.ResultRow{
			Rank: r.Rank, Name: r.Name, Score: r.Score, Left: r.Left,
		})
	}
	return out
}

// Layout adapts the logical view to the screen: the world stays CanvasH px
// tall and the visible width follows the aspect, clamped to
// ViewMinW..ViewMaxW. Ebitengine calls this on every resize, so browser
// windows and phone orientation changes re-fit live.
func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	w := float64(protocol.CanvasW)
	if outsideWidth > 0 && outsideHeight > 0 {
		w = float64(outsideWidth) / float64(outsideHeight) * protocol.CanvasH
	}
	switch {
	case w < protocol.ViewMinW:
		w = protocol.ViewMinW
	case w > protocol.ViewMaxW:
		w = protocol.ViewMaxW
	}
	g.viewW = int(math.Round(w))
	return g.viewW, protocol.CanvasH
}