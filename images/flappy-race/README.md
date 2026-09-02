# Flappy Race

A multiplayer flappy-bird-style **race** game: backend and frontend are both
Go, the frontend compiles to WebAssembly (Ebitengine) and is served by the
same binary. Every race picks a random theme — desert pyramids, clouds,
forest or underwater — and everyone flies the same pipe track; the other
players' birds are slightly transparent so you can keep your bearings.

## Run it

```sh
make run        # builds the wasm client + the server, then serves :8080
```

Open http://localhost:8080 in a few tabs (or phones on the LAN): pick a
username, and every new race starts with a synced 3-2-1 countdown.

- **Flap**: space, ↑, W, left click or tap.
- **Scoring**: +1 per pipe passed. Race ends when everyone is dead; the
  podium ranks by score, then by who survived longer.
- **Spectating**: joining mid-race (or dying) switches to a spectating view
  with a live leaderboard; you automatically fly in the next race.
- **Your bird** is random per session and persists across races (and across
  server restarts, per username, in `data/state.json`).

## Build / test

```sh
make assets     # wasm client + wasm_exec.js into internal/web/static/
make build      # server binary with embedded assets (CGO_ENABLED=0)
make native     # desktop dev client (cgo; connects to a running server)
make test       # unit + headless integration tests (real websockets, 60 Hz hub)
make test-native # client reconnect e2e: server killed/restarted under the real client
make race       # the same suite under the race detector
make vet        # vet for all three targets: host, js/wasm and native
make docker-build
```

Client-only packages carry `//go:build js && wasm || native` tags, so
host-side `go test ./...` never touches Ebitengine's desktop driver (which
is the one part that needs cgo). Everything that ships builds with
`CGO_ENABLED=0`; the `-race` test run enables cgo solely for the detector's
runtime, and `make native` opts the same client code into a desktop binary
via the `native` tag:

```sh
make run &     # server on :8080
make native
./bin/flappy-race-native -server ws://localhost:8080/ws
```

Without `-name` the window opens on the same start screen the web serves:
type a name (the all-time top 10 is fetched), Enter joins. `-name` skips
the screen for scripted runs; without `-server` the binary targets the
instance baked into `cmd/native` (`wss://birds.klimlive.de/ws`).

Plain `go run ./cmd/native/main.go` cannot work: without `-tags native`
the client package is only its host-testable files, so `client.NewGame` is
undefined (`NewGame` lives behind the tag). Use `make native`, or
`go run -tags native ./cmd/native` for a quick iteration loop.

## Architecture

```
browser (Ebitengine wasm, 800-tall world,        Go server (:8080)
  view width adapts to any screen)
  start screen: username + all-time top 10   ┌───────────────────────────┐
  input (key/click/touch) → {"t":2} flap ──▶ │ hub goroutine, no locks:   │
  60 fps render, interpolated from           │ race FSM + 60 Hz sim       │
  30 Hz JSON snapshots  ◀─────────────────── │ 30 Hz snapshot broadcast   │
                                             └───────────────────────────┘
   same binary serves index.html / game.wasm / wasm_exec.js (go:embed)
```

- **Server-authoritative**: one hub goroutine owns all state (no locks);
  clients are renderers that send flap impulses. Snapshots are idempotent —
  a client rebuilds its whole view from any single one. Dead birds stay in
  snapshots until the race ends so the dead can watch scores climb.
- **Reconnect**: losing the server (restart, network blip, idle proxy) is
  not a dead end — the client keeps re-dialing with capped backoff
  (500 ms doubling to 8 s) and re-joins under the same name, so the
  persisted bird comes back and the frozen view snaps into the new
  epoch's race. Only a server-sent rejection (invalid name) is terminal.
  `make test-native` proves it against a killed/restarted live server.
- **Countdown sync**: every snapshot carries the server wall clock (`now`)
  and the countdown target (`goAt`); clients estimate the offset with an
  EMA and never count ticks locally. Bird motion starts when the first
  `racing` snapshot arrives, not when the local number hits zero.
- **Rendering**: themes, pipes and birds are drawn procedurally from the
  shared constants in `internal/protocol` (single source of truth for game
  feel); scenery is seeded per race so all clients see identical layers.
  The view keeps the world's fixed 800 px height and adapts its width to
  the screen aspect (portrait phones → desktop → ultrawide), so it works
  on any resolution; wider screens simply see a bit further ahead.
- **Persistence**: `data/state.json` (atomic temp-file + rename) keeps the
  all-time top 50 and per-username stats + bird.

Package layout:

```
cmd/server    HTTP server + graceful shutdown
cmd/wasm      Ebitengine entry point (js/wasm only)
cmd/native    desktop dev client (build tag `native`; the one cgo target)
internal/protocol   shared leaf: wire types + ALL tuning constants
internal/game       pure 60 Hz simulation (unit-tested, no I/O)
internal/server     hub FSM, sessions, websocket pumps, JSON store
internal/client     clock/interp/HUD/reconnect logic + tagged rendering
internal/web        embedded static shell
internal/racetest   headless multi-client lifecycle test
```

Two deliberate deviations from the original plan are worth knowing:

- The **client** uses `coder/websocket` (gorilla cannot dial from WASM — its
  dialer needs `net.Dial`); the server keeps gorilla. Both are pure Go and
  interoperate over the same JSON frames.
- The desktop client is a build-tag variant of the wasm client, not a
  separate port: Ebitengine v2.9.10's Linux OpenGL driver needs cgo/GLFW,
  so `make native` is a dev-only convenience that breaks the CGO_ENABLED=0
  rule for the client binary alone. The shipped wasm/browser path stays the
  primary target (which is also what a future Android WebView or gomobile
  port builds on).

## Deployment notes (birds.klimlive.de)

The scratch image needs no runtime libraries; mount a volume at `/data`.
Behind a reverse proxy: proxy `/ws` as WebSocket (`wss://`), make sure
`.wasm` is served with `Content-Type: application/wasm`, and ideally serve
the ~18 MB client gzip/brotli-compressed — the first load without
compression is the only rough edge.

## Tuning

Every gameplay number (gravity, flap impulse, gap, scroll speed, ramp,
limits) lives in `internal/protocol/const.go` — change it there and both
sides stay in sync. Race timings (countdown/results windows) are flags:
`-addr`, `-data` plus `Config` in `internal/server/config.go`.