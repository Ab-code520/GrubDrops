package oidc

import (
	"context"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

func newTestProvider(t *testing.T, idp *fakeIDP) *Provider {
	t.Helper()
	p, err := New(context.Background(), Config{
		Issuer:       idp.srv.URL,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		RedirectURL:  "https://app.example.com/auth/oidc/callback",
		ProviderName: "Test",
	})
	require.NoError(t, err)
	return p
}

func TestNew_DisabledConfig(t *testing.T) {
	p, err := New(context.Background(), Config{})
	require.NoError(t, err)
	require.False(t, p.Enabled())
}

func TestAuthCodeURL_ContainsParams(t *testing.T) {
	idp := newFakeIDP(t)
	p := newTestProvider(t, idp)
	require.True(t, p.Enabled())
	raw := p.AuthCodeURL("state123", "nonce456", "challengeXYZ")
	u, err := url.Parse(raw)
	require.NoError(t, err)
	q := u.Query()
	require.Equal(t, "state123", q.Get("state"))
	require.Equal(t, "nonce456", q.Get("nonce"))
	require.Equal(t, "challengeXYZ", q.Get("code_challenge"))
	require.Equal(t, "S256", q.Get("code_challenge_method"))
	require.Contains(t, q.Get("scope"), "openid")
}

func TestExchangeAndVerify_Success(t *testing.T) {
	idp := newFakeIDP(t)
	idp.email = "admin@example.com"
	idp.nonce = "nonce456"
	p := newTestProvider(t, idp)

	claims, err := p.ExchangeAndVerify(context.Background(), "any-code", "verifier", "nonce456")
	require.NoError(t, err)
	require.Equal(t, "admin@example.com", claims.Email)
	require.Equal(t, "subject-123", claims.Subject)
}

func TestExchangeAndVerify_NonceMismatch(t *testing.T) {
	idp := newFakeIDP(t)
	idp.nonce = "wrong-nonce"
	p := newTestProvider(t, idp)

	_, err := p.ExchangeAndVerify(context.Background(), "any-code", "verifier", "expected-nonce")
	require.Error(t, err)
}
