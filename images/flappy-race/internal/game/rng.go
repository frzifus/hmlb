package game

// RNG is a splitmix64 generator: tiny, fast and — unlike math/rand — its
// per-seed stream is identical across Go versions and platforms, which keeps
// pipe layouts, theme picks and bird rolls reproducible everywhere.
type RNG struct{ state uint64 }

// NewRNG returns a generator seeded with s. The zero seed is valid.
func NewRNG(seed uint64) *RNG { return &RNG{state: seed} }

// Next returns the next raw 64-bit value.
func (r *RNG) Next() uint64 {
	r.state += 0x9E3779B97F4A7C15
	z := r.state
	z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
	z = (z ^ (z >> 27)) * 0x94D049BB133111EB
	return z ^ (z >> 31)
}

// Float returns a uniform value in [0, 1).
func (r *RNG) Float() float64 { return float64(r.Next()>>11) / (1 << 53) }

// Range returns a uniform value in [min, max).
func (r *RNG) Range(min, max float64) float64 { return min + (max-min)*r.Float() }