package protocol

// Theme identifies the visual world of a race. Every race picks one at
// random (never the same twice in a row); the definitions are purely
// cosmetic and live in the client's draw package.
type Theme uint8

const (
	ThemeDesert Theme = iota // pyramids over warm dunes
	ThemeClouds              // drifting cumulus bands
	ThemeForest              // tree lines and hills
	ThemeUnderwater          // kelp, coral and bubbles

	ThemeCount
)

// Valid reports whether t is a defined theme.
func (t Theme) Valid() bool { return t < ThemeCount }

// String returns the lowercase identifier used on the wire and in the
// persisted leaderboard.
func (t Theme) String() string {
	switch t {
	case ThemeDesert:
		return "desert"
	case ThemeClouds:
		return "clouds"
	case ThemeForest:
		return "forest"
	case ThemeUnderwater:
		return "underwater"
	default:
		return "unknown"
	}
}

// ThemeFromString is the inverse of String, used when reading the store.
func ThemeFromString(s string) (Theme, bool) {
	switch s {
	case "desert":
		return ThemeDesert, true
	case "clouds":
		return ThemeClouds, true
	case "forest":
		return ThemeForest, true
	case "underwater":
		return ThemeUnderwater, true
	}
	return 0, false
}