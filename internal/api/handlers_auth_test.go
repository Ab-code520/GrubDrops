package api

import (
	"bytes"
	"strings"
	"testing"

	"github.com/aalejandrofer/grubdrops/internal/web"
)

func renderLogin(t *testing.T, data templateData) string {
	t.Helper()
	tmpl, err := web.Templates()
	if err != nil {
		t.Fatalf("load templates: %v", err)
	}
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "login.html", data); err != nil {
		t.Fatalf("render login: %v", err)
	}
	return buf.String()
}

func TestLogin_SSOEnabled_ShowsPrimaryButton(t *testing.T) {
	out := renderLogin(t, templateData{OIDCEnabled: true, OIDCProviderName: "authentik"})
	if !strings.Contains(out, "Continue with SSO") {
		t.Errorf("expected generic SSO button label, got:\n%s", out)
	}
	if !strings.Contains(out, `href="/auth/oidc/login"`) {
		t.Errorf("expected SSO link to /auth/oidc/login")
	}
	if strings.Contains(out, "authentik") {
		t.Errorf("login button must NOT name the provider")
	}
	if !strings.Contains(out, `name="password"`) {
		t.Errorf("expected password field present")
	}
}

func TestLogin_SSODisabled_PasswordOnly(t *testing.T) {
	out := renderLogin(t, templateData{OIDCEnabled: false})
	if strings.Contains(out, "Continue with SSO") || strings.Contains(out, "/auth/oidc/login") {
		t.Errorf("SSO button must be hidden when OIDC disabled")
	}
	if !strings.Contains(out, `name="password"`) {
		t.Errorf("expected password field present")
	}
}
