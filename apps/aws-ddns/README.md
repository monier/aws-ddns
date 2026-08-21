# aws-ddns — technical reference

The complete reference for the daemon. For the short guide (what it is, quick start,
FAQ), read the [root README](../../README.md) first.

## Tech stack

| Concern | Technology |
|---|---|
| Language | Go (go.mod 1.25, built with the current stable toolchain) |
| AWS | AWS SDK for Go v2 (`service/route53`) |
| HTTP | standard library `net/http` (10s timeout per discovery request) |
| Logging | `log/slog`, structured JSON on stdout, level via `LOG_LEVEL` |
| Scheduling | `time.Ticker` — no cron |
| Tests | `testing` + `testify`, `go test -race` |
| Container | multi-stage Dockerfile → `scratch`, no ports; no `USER` baked in — the engine/deployment manages the user |

## Code map

| Path | Contents |
|---|---|
| `cmd/aws-ddns/` | Composition root (`main.go`), `-version` and `-data-dir` flags |
| `internal/process/` | Daemon loop: startup run, ticker, graceful shutdown, per-cycle panic recovery, cycle logging |
| `internal/services/` | `SyncService` (discover → cache check → compare → upsert) and its collaborator interfaces |
| `internal/repositories/` | HTTPS IP discoverer (fallback, IPv4 validation), the Route 53 adapter, the file state store |
| `internal/framework/` | App data folder resolution/probe, configuration (INI + env merge), the slog JSON logger |

Supporting files: `Dockerfile` (multi-stage → `scratch`; the builder cross-compiles,
so the build machine's architecture does not matter), `iam-policy.json`
(least-privilege template), `aws-ddns.ini.example` (production config template).

## Configuration

### The app data folder

Everything the app reads and writes locally lives in **one folder**: the INI file
`aws-ddns.ini` (optional) and the last-known-IP cache `last-ip.txt`. Resolution order:
`-data-dir <path>` flag → `DATA_DIR` environment variable → default
`/var/lib/aws-ddns`. At startup the folder is created when absent and probed for
read/write; a permission problem exits immediately (code 2) with a clear log entry.
`DATA_DIR` can never be set inside the INI — the file lives in the folder it would
name.

In a container, three ways to point at it (the first is almost always enough):

1. **Volume mount alone**: map a host folder to `/var/lib/aws-ddns` (the default).
2. **`DATA_DIR` env variable**: for a different path *inside* the container — set
   `DATA_DIR=/data` and mount at `/data`; the two must match.
3. **`-data-dir` argument**: same as the env (the flag wins), but container UIs bury
   command overrides; prefer the env variable.

### Sources and precedence

**defaults < INI file < environment variables**

- INI: `<data-dir>/aws-ddns.ini`; keys are the same names as the variables; `;`/`#`
  comments and cosmetic `[section]` headers allowed; **unknown keys are rejected** at
  startup so a typo fails loudly. Parsed by the app's own minimal parser — no INI
  library.
- Environment: any variable set (non-empty) overrides the file value. Empty optional
  variables fall back to defaults, so a blank `.env` line is safe.

### Settings

| Key / variable | Required | Default | Meaning |
|---|---|---|---|
| `AWS_ACCESS_KEY_ID` / `AWS_SECRET_ACCESS_KEY` | yes | — | Credentials of the dedicated IAM user (INI, or the SDK's default chain) |
| `HOSTED_ZONE_ID` | yes | — | Route 53 hosted zone id (e.g. `Z0123456789ABCDEFGHIJ`) |
| `RECORD_NAME` | yes | — | Fully-qualified record to manage (e.g. `home.example.com`) |
| `AWS_REGION` | no | `us-east-1` | SDK region (Route 53 is a global service) |
| `INTERVAL` | no | `5m` | Check interval, Go duration syntax (`90s`, `5m`, `1h`) |
| `TTL` | no | `60` | Record TTL in seconds |
| `LOG_LEVEL` | no | `info` | `verbose` \| `debug` \| `info` \| `warning` \| `error` |
| `DATA_DIR` | no | `/var/lib/aws-ddns` | App data folder (env/flag only — never an INI key) |

IP discovery endpoints are fixed, tried in order: `https://api.ipify.org`, then
`https://checkip.amazonaws.com`.

### The last-known-IP cache

After every successful check or update, the current IP is written atomically (temp +
rename) to `<data-dir>/last-ip.txt`. On the next cycle, a cache hit skips Route 53
entirely. Invariants: a failed upsert is never cached (so it retries); a
missing/corrupt/unwritable state file degrades to querying Route 53 with a warning —
only the startup probe of the folder is fatal. Trade-off: a record edited externally
while the IP is unchanged is not corrected until the IP changes (or `last-ip.txt` is
deleted).

## AWS setup — least privilege

1. In `iam-policy.json`, replace `HOSTED_ZONE_ID` with your zone id and `RECORD_NAME`
   with the record name in **lowercase** (Route 53 normalizes names).
2. Create a customer-managed policy from the file, a dedicated IAM user with **no
   console access**, attach the policy, create one access key.
3. Put the key in `aws-ddns.ini` (server) or `.env` (local).

The policy allows exactly: `UPSERT` of that one `A` record (conditioned on record
name, type, and action), plus zone-scoped `route53:ListResourceRecordSets` for the
read-before-write comparison. It cannot create/delete zones or touch other records.

## Build and development

Root Makefile targets: `init-env`, `install`, `build`, `clean`, `test`, `format`,
`lint`, `check` (full gate), `start`/`update`/`shutdown`/`prune`/`teardown` (local
Compose), `deploy` (registry publish), `export-image` (offline packs). App-level:
`install build clean test format lint image image-amd64`.

The root `VERSION` file (`<Major>.<Minor>.<Patch>`) is stamped into the binary
(`-version`) and used as the image tag; every change request bumps `Patch`
(`AGENTS.md` → Versioning).

## Local testing

The machine must reach the Internet through the **same NAT** as the target server (no
VPN, no alternate gateway). Test against a temporary record first.

```bash
export $(grep -v '^#' ../../.env | xargs)      # or set variables by hand
DATA_DIR=./data RECORD_NAME=ddns-test.example.com go run ./cmd/aws-ddns

# verify against the zone's authoritative servers (bypasses caches):
dig +short ddns-test.example.com @$(dig +short NS example.com | head -1)
curl -s https://api.ipify.org                  # should print the same address
```

## Deployment

Two modes, both multi-architecture (`amd64` + `arm64`); the container definition is
identical in both — only the image source differs.

### Registry mode (`make deploy`) — default for connected servers

Builds both architectures and pushes **one multi-arch manifest** to Docker Hub as
`docker.io/mitchmo/aws-ddns` (tags `<VERSION>` and `latest`), then renders
`dist/docker-compose.registry.yaml`. Targets pull the right architecture
automatically; pulling needs no authentication (public repository), and container UIs
find the image in their **native search** (`mitchmo/aws-ddns`) — no registry-source
configuration.

Publisher one-time setup: a Docker Hub access token (hub.docker.com → Account
settings → Personal access tokens, Read & Write), then
`podman login docker.io -u <docker-id>` (or `docker login`) with the token as
password. Docker Hub creates the repository **public by default** on first push. After
rotating a token, **log in again** — the engine caches the old credential.

Any other OCI registry works via an override, e.g.
`make deploy REGISTRY_IMAGE=ghcr.io/<github-user>/aws-ddns` (note: registries other
than Docker Hub don't appear in container UIs' search — pull by full reference there).

Upgrades: pull-based. With `:latest`, platform "update container" actions work; with a
pinned tag, publish, bump the tag, redeploy.

### Offline mode (`make export-image`) — no registry

Packs per-architecture archives (`dist/aws-ddns-<v>-linux-<arch>.tar.gz` + `.sha256`)
and renders `dist/docker-compose.yaml` pinning `localhost/aws-ddns:<v>` — the exact
name `docker load` restores.

1. Pick the archive by the target's `uname -m` (`x86_64` → amd64, `aarch64` → arm64);
   copy it with its `.sha256`.
2. `sha256sum -c <archive>.sha256`, then `docker load -i <archive>`.
3. Probe: `docker run --rm localhost/aws-ddns:<v> -version` must print the version —
   this rules out an architecture mismatch (a wrong-arch image dies instantly with no
   logs).
4. Deploy `dist/docker-compose.yaml` as a Compose project (edit the host data-folder
   line), or create the container in the UI mapping the data folder to
   `/var/lib/aws-ddns` — some platforms grant mount permissions more reliably in their
   container-creation flow than in their compose flow.

Upgrades: **recreate, never "update"/pull** — there is no registry to pull from, so
platform update actions fail (harmlessly: `unsupported manifest media type`,
`pull access denied`, or `manifest unknown`). Load the new archive, recreate the
container on the new tag (data survives in the host folder), optionally
`docker rmi` the old tag.

### Container settings (both modes)

`restart: unless-stopped`, `read_only: true`, `cap_drop: ALL`,
`no-new-privileges`, the data-folder volume — and nothing else: no ports, no
environment variables when the INI carries the configuration. Template:
[`infra/server/docker-compose.yaml`](../../infra/server/docker-compose.yaml)
(rendered into `dist/` by both make targets). Local dev Compose:
[`infra/local/docker-compose.yaml`](../../infra/local/docker-compose.yaml).

## Operations

- **Logging**: structured JSON, everything on stdout — the container's default log
  sink; read it with `docker logs` or any container UI's Logs view. Unbuffered, one
  JSON line per entry; credentials never logged.
- **Startup trail**: `starting aws-ddns` (version, os/arch, pid) → `resolving app data
  folder` (path + winning source) → `app data folder ready` → `configuration loaded`
  (INI found or not, record, interval, TTL, level) → `AWS configuration loaded`
  (region, credentials source) → `synchronization loop starting`. On a failure, the
  last line names the step that died.
- **Cycle logs**: `cycle` number, `duration`, `nextCheckIn`/`nextRetryIn`; at
  `LOG_LEVEL=debug` also each discovery endpoint (with duration), the cache
  comparison, the Route 53 read and upsert. A panic inside a cycle is caught, logged
  with its stack, and the loop keeps running; any other panic logs its stack before
  exit.
- **Exit codes**: `0` graceful shutdown or `-version`; `2` invalid configuration or an
  unusable data folder; `1` AWS configuration failure at startup. Runtime sync
  failures never exit — logged and retried next cycle.

## Troubleshooting

- **No logs at all** → the process never ran: wrong architecture (`docker image
  inspect <image> --format '{{.Architecture}}'` vs `uname -m`; probe with
  `-version`), a stale image, or a logs view that only streams live — check
  `docker inspect aws-ddns --format '{{.State.ExitCode}} {{.State.Error}} {{.RestartCount}}'`
  and `docker logs aws-ddns`.
- **`data directory … is not writable`** → the platform runs the container as a user
  that cannot write the mounted folder: grant that user write access, or pin
  `user: "<uid>:<gid>"` in compose. (`PUID`/`PGID`-style variables some platforms
  inject are ignored by this image.)
- **`unknown key "…"`** → typo in `aws-ddns.ini`.
- **Update action fails on manifest/pull errors** → offline mode has no registry;
  recreate instead (see Deployment), or switch to registry mode.
- **Sync failures** → `LOG_LEVEL=debug` and read the per-step entries (which
  endpoint, the Route 53 read, or the upsert — each with durations).
- **Stale record with unchanged IP** → delete `<data-dir>/last-ip.txt`.
