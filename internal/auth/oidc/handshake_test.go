package oidc

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

func newKVDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec(`CREATE TABLE kv (key TEXT PRIMARY KEY, value BLOB)`)
	require.NoError(t, err)
	return db
}

func TestHandshake_PutTakeRoundTrip(t *testing.T) {
	db := newKVDB(t)
	hs := NewHandshakeStore(db)
	ctx := context.Background()

	require.NoError(t, hs.Put(ctx, "state1", "nonce1", "verifier1", time.Minute))

	nonce, verifier, err := hs.Take(ctx, "state1")
	require.NoError(t, err)
	require.Equal(t, "nonce1", nonce)
	require.Equal(t, "verifier1", verifier)
}

func TestHandshake_TakeIsSingleUse(t *testing.T) {
	db := newKVDB(t)
	hs := NewHandshakeStore(db)
	ctx := context.Background()
	require.NoError(t, hs.Put(ctx, "state1", "n", "v", time.Minute))

	_, _, err := hs.Take(ctx, "state1")
	require.NoError(t, err)
	_, _, err = hs.Take(ctx, "state1")
	require.Error(t, err) // gone after first use
}

func TestHandshake_TakeRejectsExpired(t *testing.T) {
	db := newKVDB(t)
	hs := NewHandshakeStore(db)
	ctx := context.Background()
	require.NoError(t, hs.Put(ctx, "state1", "n", "v", -time.Second)) // already expired

	_, _, err := hs.Take(ctx, "state1")
	require.Error(t, err)
}

func TestHandshake_TakeUnknownState(t *testing.T) {
	db := newKVDB(t)
	hs := NewHandshakeStore(db)
	_, _, err := hs.Take(context.Background(), "nope")
	require.Error(t, err)
}
