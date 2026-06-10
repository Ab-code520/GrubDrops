package oidc

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// handshakePrefix namespaces handshake records in the shared kv table,
// distinct from the "session:" prefix used by the scs session store.
const handshakePrefix = "oidc:"

// HandshakeStore persists the per-login OIDC handshake (nonce + PKCE verifier)
// server-side, keyed by the random state value. Needed because the main
// session cookie is SameSite=Strict and is not sent on the cross-site redirect
// back from the IdP.
type HandshakeStore struct {
	db *sql.DB
}

func NewHandshakeStore(db *sql.DB) *HandshakeStore {
	return &HandshakeStore{db: db}
}

type handshakeRecord struct {
	Nonce    string `json:"nonce"`
	Verifier string `json:"verifier"`
	Expiry   int64  `json:"expiry"` // unix nano
}

// Put stores the handshake for state, expiring after ttl.
func (h *HandshakeStore) Put(ctx context.Context, state, nonce, verifier string, ttl time.Duration) error {
	rec := handshakeRecord{Nonce: nonce, Verifier: verifier, Expiry: time.Now().Add(ttl).UnixNano()}
	blob, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	_, err = h.db.ExecContext(ctx,
		`INSERT INTO kv (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		handshakePrefix+state, blob,
	)
	return err
}

// Take returns the handshake for state and deletes it (single use). It errors
// if the state is unknown or expired.
func (h *HandshakeStore) Take(ctx context.Context, state string) (nonce, verifier string, err error) {
	key := handshakePrefix + state
	var blob []byte
	err = h.db.QueryRowContext(ctx, `DELETE FROM kv WHERE key = ? RETURNING value`, key).Scan(&blob)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", fmt.Errorf("unknown oidc state")
	}
	if err != nil {
		return "", "", err
	}

	var rec handshakeRecord
	if err := json.Unmarshal(blob, &rec); err != nil {
		return "", "", err
	}
	if time.Now().UnixNano() > rec.Expiry {
		return "", "", fmt.Errorf("oidc state expired")
	}
	return rec.Nonce, rec.Verifier, nil
}
