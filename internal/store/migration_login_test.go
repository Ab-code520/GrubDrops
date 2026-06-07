package store

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/require"
)

// TestMigration0010DropsLoginAndKeepsChildren is the safety net for the
// accounts.login removal. The migration rebuilds the accounts table, and
// accounts is the FK parent of sessions/account_games/links/claims with
// ON DELETE CASCADE. If the rebuild's DROP TABLE ran with foreign_keys ON,
// the implicit delete would cascade and wipe every child row — silent data
// loss on the production DB. This migrates to v9 (login still present),
// seeds a parent + children, applies 0010, and asserts the children survive
// and the column is gone.
func TestMigration0010DropsLoginAndKeepsChildren(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "m.db")
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)&_pragma=busy_timeout(5000)", path)
	db, err := sql.Open("sqlite", dsn)
	require.NoError(t, err)
	defer db.Close()
	db.SetMaxOpenConns(1) // mirror store.Open — single pinned connection

	goose.SetBaseFS(migrationsFS)
	require.NoError(t, goose.SetDialect("sqlite3"))

	// Migrate to just before the drop (login column still exists).
	require.NoError(t, goose.UpToContext(ctx, db, "migrations", 9))

	now := time.Now().Unix()
	_, err = db.ExecContext(ctx, `INSERT INTO accounts
		(id, platform, login, display_name, status, fingerprint_json, enabled, created_at, updated_at)
		VALUES ('acc1','twitch','demo','Demo','idle','{}',1,?,?)`, now, now)
	require.NoError(t, err)
	// Child rows that would cascade-delete if the rebuild dropped the parent
	// with FK enforcement on.
	_, err = db.ExecContext(ctx, `INSERT INTO sessions (account_id, ciphertext, expires_at) VALUES ('acc1', x'00', ?)`, now+3600)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO account_games (account_id, game_id, rank) VALUES ('acc1', 'g_apex', 1)`)
	require.NoError(t, err)

	// Apply the rest (0010 drops login).
	require.NoError(t, goose.UpContext(ctx, db, "migrations"))

	var n int
	require.NoError(t, db.QueryRowContext(ctx, `SELECT count(*) FROM accounts WHERE id='acc1'`).Scan(&n))
	require.Equal(t, 1, n, "account row must survive the rebuild")
	require.NoError(t, db.QueryRowContext(ctx, `SELECT count(*) FROM sessions WHERE account_id='acc1'`).Scan(&n))
	require.Equal(t, 1, n, "session child must NOT be cascade-deleted by the rebuild")
	require.NoError(t, db.QueryRowContext(ctx, `SELECT count(*) FROM account_games WHERE account_id='acc1'`).Scan(&n))
	require.Equal(t, 1, n, "account_games child must NOT be cascade-deleted by the rebuild")

	// The login column is gone.
	_, err = db.ExecContext(ctx, `SELECT login FROM accounts`)
	require.Error(t, err, "login column should no longer exist")
}
