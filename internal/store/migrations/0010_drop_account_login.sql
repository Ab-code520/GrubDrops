-- +goose NO TRANSACTION
-- Drop the vestigial accounts.login column. The Add-Account form stopped
-- collecting it; it had degenerated into a slugified copy of display_name.
-- Identity is the account id (PK) + display_name. Removing it also drops the
-- UNIQUE(platform, login) table constraint.
--
-- login is part of a table-level UNIQUE constraint, so SQLite can't DROP
-- COLUMN it directly — we rebuild the table. Children FK to accounts(id) with
-- ON DELETE CASCADE, so foreign_keys must be OFF during the swap or dropping
-- the old table would cascade-delete every child row. NO TRANSACTION lets the
-- PRAGMA take effect (it's a no-op inside a transaction).

-- +goose Up
PRAGMA foreign_keys=OFF;
-- +goose StatementBegin
CREATE TABLE accounts_new (
    id              TEXT PRIMARY KEY,
    platform        TEXT NOT NULL,
    display_name    TEXT NOT NULL DEFAULT '',
    status          TEXT NOT NULL DEFAULT 'idle',
    proxy_url       TEXT,
    webhook_url     TEXT,
    fingerprint_json TEXT NOT NULL DEFAULT '{}',
    enabled         INTEGER NOT NULL DEFAULT 1,
    created_at      INTEGER NOT NULL,
    updated_at      INTEGER NOT NULL
);
-- +goose StatementEnd
-- +goose StatementBegin
INSERT INTO accounts_new (id, platform, display_name, status, proxy_url, webhook_url, fingerprint_json, enabled, created_at, updated_at)
SELECT id, platform, display_name, status, proxy_url, webhook_url, fingerprint_json, enabled, created_at, updated_at FROM accounts;
-- +goose StatementEnd
DROP TABLE accounts;
ALTER TABLE accounts_new RENAME TO accounts;
PRAGMA foreign_keys=ON;

-- +goose Down
ALTER TABLE accounts ADD COLUMN login TEXT NOT NULL DEFAULT '';
