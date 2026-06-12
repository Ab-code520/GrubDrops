package kick

import "testing"

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"TTik3r":       "ttik3r",
		"Phluses":      "phluses",
		"Cool_Name 99": "cool-name-99",
		"--weird--":    "weird",
		"":             "",
	}
	for in, want := range cases {
		if got := slugify(in); got != want {
			t.Errorf("slugify(%q)=%q want %q", in, got, want)
		}
	}
}

func TestSidecarName(t *testing.T) {
	tmpl := "grubdrops-browser-{slug}"
	if got := sidecarName(tmpl, "TTik3r"); got != "grubdrops-browser-ttik3r" {
		t.Fatalf("got %q", got)
	}
	// empty slug → empty name (caller treats as "no controllable sidecar")
	if got := sidecarName(tmpl, ""); got != "" {
		t.Fatalf("empty username should yield empty name, got %q", got)
	}
}
