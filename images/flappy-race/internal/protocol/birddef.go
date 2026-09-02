package protocol

// Bird is a player's appearance: a color palette plus a little accessory.
// It is assigned per session (randomly for new usernames, restored from the
// store for known ones) and stays constant until the player disconnects.
type Bird struct {
	Palette   uint8 `json:"p"` // 0..PaletteCount-1, body/wing/beak colors
	Accessory uint8 `json:"a"` // 0..AccessoryCount-1, cosmetic headgear
}

const (
	PaletteCount   = 12
	AccessoryCount = 4 // accessory 0 means "none"; the client owns the looks
)

// Valid reports whether the bird's values are in range.
func (b Bird) Valid() bool {
	return b.Palette < PaletteCount && b.Accessory < AccessoryCount
}

// RandomBird returns a random valid bird. avoid, when non-nil, is consulted
// so that visually identical birds are rare while a lobby fills up; pass a
// function reporting whether a candidate is already taken.
func RandomBird(next func() uint64, taken func(Bird) bool) Bird {
	for range 32 {
		b := Bird{
			Palette:   uint8(next() % PaletteCount),
			Accessory: uint8(next() % AccessoryCount),
		}
		if taken == nil || !taken(b) {
			return b
		}
	}
	// Everything is taken (huge lobby) — duplicates are fine then.
	return Bird{
		Palette:   uint8(next() % PaletteCount),
		Accessory: uint8(next() % AccessoryCount),
	}
}