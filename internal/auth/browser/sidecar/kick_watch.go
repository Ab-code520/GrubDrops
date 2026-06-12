package sidecar

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"

	pb "github.com/aalejandrofer/grubdrops/internal/auth/browser/gen/browser/v1"
)

// Kick credits drop watch-time ONLY for a real, actively-playing IVS
// video session (proven: a CDP-driven Chrome playing kick.com/<channel>
// accrued progress; the pure-HTTP utls viewer-WS does NOT). This file
// drives a headless Chrome tab through the player so watch-time accrues.
//
// Resource notes: one video-playing tab per active watch. We mute it and
// pin the lowest available quality to keep CPU/bandwidth down. Because
// watching ANY live channel in a campaign's category accrues ALL
// same-category open campaigns at once, the caller should keep at most
// one watch tab per Kick account.

const (
	// kickWatchSettleWait is how long we give the SPA + IVS player to
	// mount the <video> element after navigation before driving it.
	kickWatchSettleWait = 8 * time.Second
	// kickWatchKeepAliveEvery is the cadence of the background nudge that
	// un-pauses / re-mutes the player if Kick's UI or an ad break paused
	// it. Kick accrues per-minute, so a sub-minute nudge is plenty.
	kickWatchKeepAliveEvery = 20 * time.Second
)

// kickPlayerDriveScript locates the IVS <video>, mutes it, sets the
// lowest quality if a quality menu is reachable, and calls play(). It is
// idempotent and safe to run repeatedly (used for both the initial drive
// and the keep-alive nudge). Returns a small JSON status string for
// logging/liveness. Pure DOM/JS — no element-selector brittleness beyond
// the <video> tag itself, which IVS always renders.
const kickPlayerDriveScript = `(() => {
  const out = {video: false, playing: false, muted: false, readyState: -1, currentTime: 0};
  try {
    const v = document.querySelector('video');
    if (!v) return JSON.stringify(out);
    out.video = true;
    // Mute first so autoplay policies don't block play().
    try { v.muted = true; v.volume = 0; } catch (e) {}
    out.muted = !!v.muted;
    out.readyState = v.readyState;
    out.currentTime = v.currentTime || 0;
    if (v.paused) {
      const p = v.play();
      if (p && typeof p.catch === 'function') { p.catch(() => {}); }
    }
    out.playing = !v.paused && !v.ended && v.readyState >= 2;
    // Best-effort lowest-quality: Kick exposes a settings/quality gear in
    // its custom player. We can't reliably click it headless, but if the
    // page wires a global hook we use it. Most builds don't, so this is a
    // no-op fallback — the muted low-volume tab is already cheap.
    try {
      if (window.__kickPlayer && typeof window.__kickPlayer.setQuality === 'function') {
        window.__kickPlayer.setQuality('lowest');
      }
    } catch (e) {}
  } catch (e) {
    out.err = String(e);
  }
  return JSON.stringify(out);
})()`

// OpenStreamWatch opens kick.com/<channel> in a fresh tab with the
// account's cookies injected, settles past Cloudflare, drives the IVS
// player to muted/playing, and starts a keep-alive goroutine that nudges
// the player every kickWatchKeepAliveEvery. Returns the tab handle (used
// as the watch id) so Heartbeat/StopWatch can target it.
//
// This supersedes the fire-and-forget OpenStream for the browser-watch
// path. OpenStream is retained for callers that only need a passive tab.
func (k *Kick) OpenStreamWatch(channel string, session *pb.KickSession) (string, error) {
	if channel == "" {
		return "", fmt.Errorf("kick watch: empty channel")
	}
	handle, ctx, err := k.b.OpenTab()
	if err != nil {
		return "", err
	}
	// Install stealth + cookies before navigation.
	if err := chromedp.Run(ctx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			_, err := page.AddScriptToEvaluateOnNewDocument(StealthScript).Do(ctx)
			return err
		}),
	); err != nil {
		k.b.CloseTab(handle)
		return "", fmt.Errorf("kick watch install stealth: %w", err)
	}
	if err := k.InstallCookies(ctx, session); err != nil {
		k.b.CloseTab(handle)
		return "", fmt.Errorf("kick watch install cookies: %w", err)
	}
	if err := chromedp.Run(ctx,
		chromedp.Navigate(fmt.Sprintf("https://kick.com/%s", channel)),
		chromedp.Sleep(3*time.Second),
	); err != nil {
		k.b.CloseTab(handle)
		return "", fmt.Errorf("kick watch navigate %s: %w", channel, err)
	}
	if err := waitCloudflareSettled(ctx, 20*time.Second); err != nil {
		slog.Warn("kick watch: cloudflare interstitial did not settle", "channel", channel, "err", err.Error())
	}
	// Let the SPA mount the player, then drive it.
	var status string
	if err := chromedp.Run(ctx,
		chromedp.Sleep(kickWatchSettleWait),
		chromedp.Evaluate(kickPlayerDriveScript, &status),
	); err != nil {
		k.b.CloseTab(handle)
		return "", fmt.Errorf("kick watch drive player %s: %w", channel, err)
	}
	slog.Info("kick watch opened", "channel", channel, "handle", handle, "player", status)

	// Keep-alive: re-drive the player on an interval so an ad break, a
	// stall, or Kick's UI pausing on tab-blur doesn't silently stop
	// accrual. Bound to the tab context so it exits when the tab closes.
	go k.watchKeepAlive(ctx, channel, handle)
	return handle, nil
}

// watchKeepAlive periodically re-runs the player drive script for an open
// watch tab. Exits when the tab context is cancelled (StopWatch / browser
// close) or when the tab can no longer be driven.
func (k *Kick) watchKeepAlive(ctx context.Context, channel, handle string) {
	t := time.NewTicker(kickWatchKeepAliveEvery)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			// Tab may have been closed by StopWatch between ticks.
			if _, ok := k.b.Tab(handle); !ok {
				return
			}
			var status string
			driveCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			err := chromedp.Run(driveCtx, chromedp.Evaluate(kickPlayerDriveScript, &status))
			cancel()
			if err != nil {
				slog.Debug("kick watch keepalive drive failed", "channel", channel, "handle", handle, "err", err)
				continue
			}
			slog.Debug("kick watch keepalive", "channel", channel, "handle", handle, "player", status)
		}
	}
}

// WatchAlive reports whether the watch tab still exists AND its <video>
// element is actually playing (not merely that the tab is open). Used by
// the Heartbeat RPC so the watcher swaps channels when playback dies
// (stream ended, player errored) rather than holding a dead tab.
func (k *Kick) WatchAlive(handle string) bool {
	ctx, ok := k.b.Tab(handle)
	if !ok {
		return false
	}
	var status string
	probeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := chromedp.Run(probeCtx, chromedp.Evaluate(kickPlayerProbeScript, &status)); err != nil {
		// A failed probe doesn't necessarily mean the watch is dead (a
		// transient CDP hiccup), but the tab is unusable for accrual right
		// now — report not-alive so the watcher re-picks.
		slog.Debug("kick watch probe failed", "handle", handle, "err", err)
		return false
	}
	return parseWatchAlive(status)
}

// kickPlayerProbeScript reports whether the IVS <video> is currently
// playing. Read-only (no play()/mute side effects) so Heartbeat stays a
// pure check; the keep-alive goroutine owns the corrective nudges.
const kickPlayerProbeScript = `(() => {
  try {
    const v = document.querySelector('video');
    if (!v) return JSON.stringify({video: false, playing: false});
    const playing = !v.paused && !v.ended && v.readyState >= 2;
    return JSON.stringify({video: true, playing: playing, readyState: v.readyState});
  } catch (e) {
    return JSON.stringify({video: false, playing: false, err: String(e)});
  }
})()`

// parseWatchAlive interprets the probe script's JSON. "Alive" means the
// <video> exists and is playing. A missing video or a paused/errored
// player counts as not-alive so the watcher re-picks a channel.
func parseWatchAlive(status string) bool {
	if status == "" {
		return false
	}
	var s struct {
		Video   bool `json:"video"`
		Playing bool `json:"playing"`
	}
	if err := json.Unmarshal([]byte(status), &s); err != nil {
		return false
	}
	return s.Video && s.Playing
}
