package gameslug

import "testing"

func TestSlug(t *testing.T) {
	cases := map[string]string{
		"":                     "",
		"Apex Legends":         "apex-legends",
		"Counter-Strike 2":     "counter-strike-2",
		"World of Warcraft":    "world-of-warcraft",
		"Dead by Daylight":     "dead-by-daylight",
		"Dota 2":               "dota-2",
		"Tom's Game!!":         "toms-game",
		"  spaced  out  ":      "spaced-out",
		"under_score_name":     "under-score-name",
		"--leading-trailing--": "leading-trailing",
		"!!!":                  "",
	}
	for in, want := range cases {
		if got := Slug(in); got != want {
			t.Errorf("Slug(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestID(t *testing.T) {
	cases := map[string]string{
		"Apex Legends":     "g_apex_legends",
		"Counter-Strike 2": "g_counter_strike_2",
		"":                 "g_",
	}
	for in, want := range cases {
		if got := ID(in); got != want {
			t.Errorf("ID(%q) = %q, want %q", in, got, want)
		}
	}
}
