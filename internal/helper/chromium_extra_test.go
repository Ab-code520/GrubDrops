package helper

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestExtraChromiumCookieDBs_IncludesOperaGX(t *testing.T) {
	dbs := extraChromiumCookieDBs()
	if len(dbs) == 0 {
		t.Fatal("no candidate cookie DBs returned for this OS")
	}
	foundGX := false
	for _, p := range dbs {
		// Match the OS-specific dir names: macOS "com.operasoftware.OperaGX",
		// Linux "opera-gx", Windows "Opera GX".
		lp := strings.ToLower(p)
		if strings.Contains(lp, "operagx") || strings.Contains(lp, "opera gx") || strings.Contains(lp, "opera-gx") {
			foundGX = true
		}
		if filepath.Base(p) != "Cookies" {
			t.Errorf("candidate should point at a Cookies DB, got %s", p)
		}
	}
	if !foundGX {
		t.Errorf("Opera GX cookie DB missing from candidates: %v", dbs)
	}
}
