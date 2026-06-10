package oidc

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
)

// randomString returns a URL-safe base64 token with at least n bytes of
// entropy.
func randomString(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic("oidc: crypto/rand failed: " + err.Error())
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// pkceChallenge computes the S256 PKCE code challenge for a verifier.
func pkceChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// NewState returns a random state or nonce value (32 bytes of entropy).
func NewState() string { return randomString(32) }

// NewVerifier returns a PKCE code verifier.
func NewVerifier() string { return randomString(48) }

// Challenge returns the S256 PKCE challenge for a verifier.
func Challenge(verifier string) string { return pkceChallenge(verifier) }
