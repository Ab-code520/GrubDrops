# On-demand Kick sidecars — stop browser containers when idle

**Date:** 2026-06-12
**Status:** Approved (design), pending implementation plan
**Branch:** builds on `agent-a12f334b3f499cfa9` (per-account sidecar pool, commit `7fe74ac`)

## Problem

Each Kick account watches via its own chromedp sidecar (a real IVS `<video>`
playing Chrome ≈ 0.4–0.6 GiB RAM). The host has no swap. Today both sidecars
run 24/7 even when there is nothing to mine — between Kick drop events all
campaigns are expired, yet two Chromes sit idle holding ~1 GiB.

Detection (campaigns, eligible-channel liveness, progress) is 100% pure-HTTP
over the utls transport and needs no browser. Only `StartWatch`/`Heartbeat`
(the IVS playback) need the sidecar. So the daemon can keep polling with
sidecars stopped and only spin one up when there is actually a live channel to
watch for that account.

## Goal

Per-account: stop an account's sidecar container when that account has nothing
watchable for a grace period; start it on demand when a watchable live channel
appears. Free Chrome RAM during lulls and entirely between events. Never block
or break mining; degrade gracefully if Docker control is unavailable.

## Non-goals

- No change to the accrual mechanism (still browser IVS playback).
- No change to Twitch (utls/device-code, no browser).
- No autoscaling beyond the configured per-account sidecars.
- Not `docker pause` (that freezes the process but keeps RAM resident). We use
  `docker stop` / `docker start` to actually reclaim memory.

## Configuration

Per-account sidecar identity is **auto-derived from the account username** via a
template, so adding an account needs no env edit — only a matching compose
container.

```
GRUB_KICK_SIDECAR_TEMPLATE=grubdrops-browser-{slug}   # default; port appended as :9090
GRUB_KICK_SIDECAR_PORT=9090                            # default
```

- `slug` = the account's `display_name` lowercased, with every char outside
  `[a-z0-9-]` collapsed to `-` and runs trimmed (e.g. `TTik3r`→`ttik3r`,
  `Phluses`→`phluses`). Deterministic.
- The derived string `grubdrops-browser-<slug>` is used for ALL THREE: the gRPC
  dial host (compose DNS), `docker start`, and `docker stop`. The port forms the
  gRPC URL `…-<slug>:9090`.
- **Coupling (must hold):** the compose `container_name` of each sidecar MUST
  equal the derived slug. Drift = daemon dials/controls a nonexistent name.
  Deterministic slugify + operator-controlled compose keeps them in lockstep.
- gRPC dial is lazy (`grpc.NewClient` connects on first RPC), so a stopped
  on-demand container is fine until the first `StartWatch`.
- On-demand stop/start is ENABLED when `GRUB_KICK_BROWSER_WATCH=1` AND the
  docker socket is controllable (see degrade path). When the socket is
  unavailable the daemon still dials the derived URLs but never stops them
  (always-on, today's behavior).
- Fallback (no regression): the existing `GRUB_BROWSER_URLS` / `GRUB_BROWSER_URL`
  remain the login / Twitch / display client and the watch path if no per-account
  template resolves.

## Components

### 1. `dockerctl` (new, `internal/dockerctl`)
Thin wrapper over the Docker Engine SDK (`github.com/docker/docker/client`),
talking to a mounted `/var/run/docker.sock`. Single purpose: control a
container by name.

- `New() (*Client, error)` — `client.NewClientWithOpts(FromEnv, WithAPIVersionNegotiation)`. Returns error if the socket is unreachable.
- `Start(ctx, name) error` — idempotent (`ContainerStart`; no-op if already running).
- `Stop(ctx, name, timeout) error` — idempotent (`ContainerStop`).
- `Running(ctx, name) (bool, error)` — `ContainerInspect`, reads `State.Running`.

Testable against the interface; no daemon logic inside.

### 2. Kick backend sidecar registry (extend `internal/platform/kick`)
Replace the anonymous `watchPool`/`clientByAcc` round-robin with a registry
keyed by accountID, each entry derived from the account username via the
template (slug → `grubdrops-browser-<slug>` + port). The backend is told each
Kick account's `(accountID, username)` at registration (the login handler and
startup account enumeration already have both):

```
type sidecar struct {
    grpcURL       string
    containerName string
    client        *browser.Client
    mu            sync.Mutex
    idleSince     time.Time // zero = not idle
}
```

- `accountID -> *sidecar` map, set at construction.
- `watchClientFor(accountID)` returns the account's pinned client (unchanged
  contract for `StartWatch`); when a per-account sidecar is derived the
  client is still dialed once at startup and reused (gRPC reconnects across
  container restarts on its own).

### 3. On-demand lifecycle hook
The backend gains two methods the watcher path calls:

- `EnsureSidecarUp(ctx, accountID) error` — called by `StartWatch` BEFORE the
  gRPC `StartWatch`. If a controllable sidecar is configured: `dockerctl.Start`,
  then poll for gRPC readiness (a cheap `Heartbeat("")` or a dedicated health
  ping) up to `startTimeout` (~30s). Clears `idleSince`. No-op (returns nil) when
  on-demand is not enabled.
- `MarkIdle(accountID)` / `MarkActive(accountID)` — the watcher reports, each
  discovery tick, whether the account currently has something watchable. Backend
  stamps/clears `idleSince`.

A single background reaper goroutine (started in `New` when on-demand enabled)
ticks every minute: for each sidecar with `idleSince` older than `idleGrace`
(10 min) and currently running, call `dockerctl.Stop` and log it.

### 4. Watcher integration
Minimal: the watcher already knows per-account whether it found a watchable
campaign+channel (it transitions to `pick_stream`/`watching`) versus nothing
(it idles in `pick_campaign`). On each discovery outcome it calls the backend's
`MarkActive`/`MarkIdle`. `StartWatch` internally calls `EnsureSidecarUp`. No
new watcher state machine.

## Data flow

```
watcher discovery (HTTP) ── watchable? ──► MarkActive ──► StartWatch
                                                          └─► EnsureSidecarUp: docker start + readiness ─► gRPC StartWatch ─► IVS plays
                          └─ nothing ────► MarkIdle (stamp idleSince)

reaper (1/min): idleSince > 10min && running ──► docker stop ──► RAM freed
```

## Error handling / graceful degrade

- `dockerctl.New` fails (no socket / no perms): log once at startup, disable
  on-demand control, leave sidecars always-on (today's behavior). Mining
  unaffected.
- `docker start` fails or readiness times out: surface as a normal `StartWatch`
  error → existing watcher backoff. Reaper will retry start on the next watch.
- `docker stop` fails: log, retry next reaper tick.
- Reaper never stops a sidecar that is currently `watching` (guarded by
  `idleSince` only being set when the account has nothing watchable).

## Deployment

- `compose.yml`: rename `grubdrops-browser` → `grubdrops-browser-ttik3r`,
  `grubdrops-browser2` → `grubdrops-browser-phluses` (same image, same
  `expose: 9090`). Add `/var/run/docker.sock:/var/run/docker.sock:ro`? — NO:
  start/stop needs write, mount read-write. `grubdrops` gets the socket mount.
- `.env`: no new per-account vars needed — names auto-derive (`grubdrops-browser-ttik3r`,
  `grubdrops-browser-phluses`). Optionally set `GRUB_KICK_SIDECAR_TEMPLATE` to
  override the default. `GRUB_BROWSER_URLS` can be dropped.
- `depends_on` updated to the renamed services. Sidecars keep
  `restart: unless-stopped`; the daemon's `docker stop` overrides until the
  next `docker start` (a manual `compose up` won't fight it because stop is
  explicit, not a crash).
- Security note: mounting the docker socket grants the daemon root-equivalent
  control of the host's Docker. Accepted for this homelab; documented here.

## Testing

- `dockerctl`: unit tests against a fake Docker client interface (Start/Stop
  idempotent, Running parses state, errors propagate).
- Kick registry: slugify usernames (mixed case, spaces/symbols, empty →
  fallback), derived URL/container name correct, `watchClientFor` pins correctly.
- Lifecycle: with a fake `dockerctl`, assert `EnsureSidecarUp` starts + waits;
  `MarkIdle` then reaper after grace calls Stop; `MarkActive` clears idle and
  reaper does NOT stop; degrade path when dockerctl is nil.
- Live (manual, prod): expire/lull → both containers `docker stop` after 10
  min; a live channel reappears → container starts, player plays, progress
  advances; RAM drops to ~daemon-only between events.

## Rollout

Build on the `agent-a12f334b3f499cfa9` worktree (deployed source). Deploy via
the standard COPY-only image + `compose up -d --force-recreate`. Verify both
accounts still mine, then force an idle window to confirm stop/start.
