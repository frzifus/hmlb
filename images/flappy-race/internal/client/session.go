package client

import "sync"

// Sess is the local player's identity, written by the network goroutine on
// the welcome frame and read by the render loop.
type Sess struct {
	mu   sync.Mutex
	id   uint64
	name string
	bird [2]uint8 // palette, accessory
	spect bool
}

func (s *Sess) Set(id uint64, name string, pal, acc uint8, spect bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.id, s.name, s.spect = id, name, spect
	s.bird = [2]uint8{pal, acc}
}

func (s *Sess) Get() (id uint64, pal, acc uint8) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.id, s.bird[0], s.bird[1]
}

// Name returns the confirmed (server-deduped) username.
func (s *Sess) Name() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.name
}