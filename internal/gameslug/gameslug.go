// Package gameslug normalizes game names into the canonical slug + id forms
// used across the app (Twitch directory lookups, the games table, campaign
// persistence). Previously each package carried its own copy (twitch
// slugify, api slugifyGame, store slugFromName/gameIDFromName); they could
// drift on edge cases. This is the single source of truth.
package gameslug

import "strings"

// Slug normalizes a game name to its canonical dash slug:
//
//	"Apex Legends"      -> "apex-legends"
//	"Counter-Strike 2"  -> "counter-strike-2"
//	"Tom's Game!!"      -> "toms-game"
//
// Lowercases; runs of space / '-' / '_' collapse to a single dash; every
// other character (apostrophes, periods, colons, ...) is dropped; leading
// and trailing dashes are trimmed.
func Slug(name string) string {
	out := make([]byte, 0, len(name))
	for i := 0; i < len(name); i++ {
		c := name[i]
		switch {
		case c >= 'A' && c <= 'Z':
			out = append(out, c+32)
		case (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9'):
			out = append(out, c)
		case c == ' ' || c == '-' || c == '_':
			if len(out) > 0 && out[len(out)-1] != '-' {
				out = append(out, '-')
			}
		}
		// everything else is dropped
	}
	for len(out) > 0 && out[len(out)-1] == '-' {
		out = out[:len(out)-1]
	}
	return string(out)
}

// ID returns the internal game id: "g_" + the slug with dashes as
// underscores ("Apex Legends" -> "g_apex_legends"). The g_ prefix keeps
// generated ids from colliding with user-entered ones.
func ID(name string) string {
	return "g_" + strings.ReplaceAll(Slug(name), "-", "_")
}
