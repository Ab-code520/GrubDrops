-- +goose Up
-- +goose StatementBegin
-- Per-account profile picture. Twitch stores the direct static-cdn.jtvnw.net
-- URL (public CDN, embedded as-is); Kick stores its profile_pic value, served
-- through the /img/kick proxy so Cloudflare doesn't 403 the hotlink. Empty
-- string means "no avatar fetched yet" — the UI falls back to the letter
-- circle. Populated on login + refreshed by the periodic auth-health sweep.
ALTER TABLE accounts ADD COLUMN avatar_url TEXT NOT NULL DEFAULT '';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE accounts DROP COLUMN avatar_url;
-- +goose StatementEnd
