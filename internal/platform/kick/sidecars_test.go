package kick

import (
	"context"
	"sync"
	"testing"
	"time"
)

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

// fakeCtl implements dockerctl.Controller for tests.
type fakeCtl struct {
	mu     sync.Mutex
	run    map[string]bool
	starts []string
	stops  []string
}

func newFakeCtl() *fakeCtl { return &fakeCtl{run: map[string]bool{}} }
func (f *fakeCtl) Start(_ context.Context, n string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.run[n] = true
	f.starts = append(f.starts, n)
	return nil
}
func (f *fakeCtl) Stop(_ context.Context, n string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.run[n] = false
	f.stops = append(f.stops, n)
	return nil
}
func (f *fakeCtl) Running(_ context.Context, n string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.run[n], nil
}
func (f *fakeCtl) stopCount() int { f.mu.Lock(); defer f.mu.Unlock(); return len(f.stops) }

func TestRegistry_ReaperStopsIdleRunning(t *testing.T) {
	ctl := newFakeCtl()
	ctl.run["grubdrops-browser-ttik3r"] = true
	reg := newSidecarRegistry(ctl, "grubdrops-browser-{slug}", 9090, 50*time.Millisecond)
	reg.register("acc1", "TTik3r")
	// lastActive far in the past → reaper should stop it.
	reg.touchAt("acc1", time.Now().Add(-time.Hour))

	reg.reapOnce(context.Background())
	if ctl.stopCount() != 1 {
		t.Fatalf("want 1 stop, got %d", ctl.stopCount())
	}
}

func TestRegistry_ReaperKeepsFreshAndStopped(t *testing.T) {
	ctl := newFakeCtl()
	ctl.run["grubdrops-browser-ttik3r"] = true   // fresh, running
	ctl.run["grubdrops-browser-phluses"] = false // idle but already stopped
	reg := newSidecarRegistry(ctl, "grubdrops-browser-{slug}", 9090, 50*time.Millisecond)
	reg.register("acc1", "TTik3r")
	reg.register("acc2", "Phluses")
	reg.touchAt("acc1", time.Now())                 // fresh
	reg.touchAt("acc2", time.Now().Add(-time.Hour)) // idle but stopped

	reg.reapOnce(context.Background())
	if ctl.stopCount() != 0 {
		t.Fatalf("want 0 stops, got %d", ctl.stopCount())
	}
}

func TestRegistry_NilControllerDegrades(t *testing.T) {
	reg := newSidecarRegistry(nil, "grubdrops-browser-{slug}", 9090, time.Minute)
	reg.register("acc1", "TTik3r")
	// must not panic / must be a no-op
	if err := reg.ensureUp(context.Background(), "acc1", func(context.Context) error { return nil }); err != nil {
		t.Fatalf("nil controller ensureUp: %v", err)
	}
	reg.reapOnce(context.Background())
}
