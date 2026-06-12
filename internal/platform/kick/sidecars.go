package kick

import "strings"

// slugify lowercases s and collapses every run of chars outside [a-z0-9] to a
// single '-', trimming leading/trailing '-'. Deterministic so the derived
// container name always matches what compose declares.
func slugify(s string) string {
	var b strings.Builder
	prevDash := false
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			prevDash = false
		} else if !prevDash {
			b.WriteByte('-')
			prevDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

// sidecarName fills {slug} in the template from the username. Returns "" when
// the username yields an empty slug (caller treats as no controllable sidecar).
func sidecarName(template, username string) string {
	slug := slugify(username)
	if slug == "" {
		return ""
	}
	return strings.ReplaceAll(template, "{slug}", slug)
}
