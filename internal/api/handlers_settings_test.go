package api

import (
	"bytes"
	"context"
	"errors"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aalejandrofer/grubdrops/internal/store"
	"github.com/aalejandrofer/grubdrops/internal/store/gen"
	"github.com/aalejandrofer/grubdrops/internal/web"
)

func TestSettingsTemplateRenders(t *testing.T) {
	tmpl, err := web.Templates()
	if err != nil {
		t.Fatalf("load templates: %v", err)
	}
	var buf bytes.Buffer
	err = tmpl.ExecuteTemplate(&buf, "settings.html", templateData{
		AuthedAdmin: true, CSRFToken: "tok", Active: "settings",
		Page: settingsPageData{
			GlobalDiscordWebhook: "https://discord.com/api/webhooks/x",
			NotifyAvatarURL:      "https://img/a.png",
			NotifyClaim:          true,
		},
	})
	if err != nil {
		t.Fatalf("render settings.html: %v", err)
	}
	out := buf.String()
	for _, want := range []string{
		`name="notify_avatar_url"`,        // avatar input wired
		`https://img/a.png`,               // its value rendered
		`hx-post="/settings/notify-test"`, // test button wired
		`id="notify-test-result"`,         // result target present
	} {
		if !strings.Contains(out, want) {
			t.Errorf("settings.html missing %q", want)
		}
	}
}

type fakeNotifier struct {
	calls int
	last  map[string]any
	err   error
}

func (f *fakeNotifier) Notify(_ context.Context, _ string, fields map[string]any) error {
	f.calls++
	f.last = fields
	return f.err
}

// newTestSettings spins up a migrated sqlite-backed settings store + queries.
func newTestSettings(t *testing.T) (*store.Settings, *gen.Queries) {
	t.Helper()
	db, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	q := gen.New(db)
	return store.NewSettings(q), q
}

func TestNotifyTest_FiresSampleAndReportsOK(t *testing.T) {
	s, q := newTestSettings(t)
	if err := s.SetGlobalDiscordWebhook(context.Background(), "https://discord/x"); err != nil {
		t.Fatal(err)
	}
	fn := &fakeNotifier{}
	d := &settingsDeps{notifier: fn, s: s, q: q}

	rec := httptest.NewRecorder()
	d.notifyTest(rec, httptest.NewRequest("POST", "/settings/notify-test", nil))

	if fn.calls != 1 {
		t.Fatalf("expected notifier called once, got %d", fn.calls)
	}
	if got := strings.ToLower(rec.Body.String()); !strings.Contains(got, "sent") {
		t.Fatalf("expected success fragment, got %q", rec.Body.String())
	}
	// Sample must carry the rich fields so the operator sees a real-looking embed.
	for _, k := range []string{"game", "drop", "channel", "platform", "req_min"} {
		if _, ok := fn.last[k]; !ok {
			t.Errorf("sample event missing %q field", k)
		}
	}
}

func TestNotifyTest_ReportsErrorFromNotifier(t *testing.T) {
	s, q := newTestSettings(t)
	_ = s.SetGlobalDiscordWebhook(context.Background(), "https://discord/x")
	fn := &fakeNotifier{err: errors.New("webhook 404")}
	d := &settingsDeps{notifier: fn, s: s, q: q}

	rec := httptest.NewRecorder()
	d.notifyTest(rec, httptest.NewRequest("POST", "/settings/notify-test", nil))

	if got := rec.Body.String(); !strings.Contains(got, "webhook 404") {
		t.Fatalf("expected error surfaced, got %q", got)
	}
}

func TestNotifyTest_NoWebhookConfigured(t *testing.T) {
	// Notifier wired, but no global webhook and no account webhooks → must
	// report honestly and NOT call the notifier (avoids silent Noop success).
	s, q := newTestSettings(t)
	fn := &fakeNotifier{}
	d := &settingsDeps{notifier: fn, s: s, q: q}

	rec := httptest.NewRecorder()
	d.notifyTest(rec, httptest.NewRequest("POST", "/settings/notify-test", nil))

	if fn.calls != 0 {
		t.Fatalf("notifier should not fire with no webhook, got %d calls", fn.calls)
	}
	if got := strings.ToLower(rec.Body.String()); !strings.Contains(got, "no webhook") {
		t.Fatalf("expected 'no webhook' message, got %q", rec.Body.String())
	}
}

func TestNotifyTest_NoNotifierConfigured(t *testing.T) {
	d := &settingsDeps{notifier: nil}
	rec := httptest.NewRecorder()
	d.notifyTest(rec, httptest.NewRequest("POST", "/settings/notify-test", nil))
	if got := strings.ToLower(rec.Body.String()); !strings.Contains(got, "no notifier") {
		t.Fatalf("expected 'no notifier' message, got %q", rec.Body.String())
	}
}

func TestSettings_SSOCard_Enabled(t *testing.T) {
	tmpl, err := web.Templates()
	if err != nil {
		t.Fatalf("load templates: %v", err)
	}
	var buf bytes.Buffer
	err = tmpl.ExecuteTemplate(&buf, "settings.html", templateData{
		Page: settingsPageData{
			OIDC: settingsOIDC{
				Enabled:      true,
				ProviderName: "authentik",
				Issuer:       "https://auth.ryuzec.dev/application/o/grubdrops/",
				CallbackURL:  "https://drops.ryuzec.dev/auth/oidc/callback",
			},
		},
	})
	if err != nil {
		t.Fatalf("render settings: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"Single sign-on", "authentik", "auth.ryuzec.dev", "drops.ryuzec.dev/auth/oidc/callback"} {
		if !strings.Contains(out, want) {
			t.Errorf("settings missing %q", want)
		}
	}
}

func TestSettings_SSOCard_Disabled(t *testing.T) {
	tmpl, err := web.Templates()
	if err != nil {
		t.Fatalf("load templates: %v", err)
	}
	var buf bytes.Buffer
	err = tmpl.ExecuteTemplate(&buf, "settings.html", templateData{
		Page: settingsPageData{OIDC: settingsOIDC{Enabled: false}},
	})
	if err != nil {
		t.Fatalf("render settings: %v", err)
	}
	if !strings.Contains(buf.String(), "Not configured") {
		t.Errorf("expected disabled SSO card to show 'Not configured'")
	}
}
