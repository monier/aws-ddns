# aws-ddns

A small Dynamic DNS service that keeps an AWS Route 53 `A` record synchronized with the
current public IPv4 address of the network where it runs — built for Docker-capable
local servers behind a residential router with a dynamic public IP.

**Author:** Monier R. · **License:** [MIT](LICENSE) · **Distribution:** this version
targets a local, registry-less build — you build the image yourself and import it
manually on the target (see Deployment).

This README is the single comprehensive document for the repository: context,
prerequisites, configuration, build, testing, deployment, and operations.
`apps/aws-ddns/README.md` stays minimal on purpose.

---

## How it works

Every cycle (once at startup, then every `INTERVAL`, default 5 minutes):

1. **Discover** the current public IPv4 over HTTPS — `https://api.ipify.org` first,
   falling back to `https://checkip.amazonaws.com` (10s timeout per request, response
   validated as an IPv4).
2. **Compare with the local cache** — the last synchronized IP stored in the app data
   folder. If unchanged, the cycle ends here: **Route 53 is not queried at all**.
3. **Compare with Route 53** — read the configured `A` record; if it is missing or holds
   a different address, `UPSERT` it with the configured `TTL`.
4. **Persist** the address to the cache — only after a successful check/update; a failed
   upsert is never cached, so it is retried next cycle.

Resilience: transient discovery or AWS failures are logged and the loop keeps running; a
missing/corrupt/unwritable cache degrades to querying Route 53 (warning, never a crash);
`SIGTERM`/`SIGINT` shut the daemon down gracefully. The service exposes **no inbound
port**, needs **no router integration**, and uses **no database**.

Known trade-off: if the record is changed externally while the public IP stays the same,
the daemon won't correct it until the IP next changes (or the cache file is deleted).

Architecture (Layered — `process` → `services` → `repositories` / `framework`) is
diagrammed in `docs/architecture.md`; rules live in `AGENTS.md`.

---

## Repository layout

| Path | Description |
|---|---|
| `apps/aws-ddns/` | The daemon — Go source, Dockerfile, IAM policy, INI template |
| `docs/` | Architecture diagram; `database.md` records that no database exists |
| `infra/local/` | Local Docker Compose environment |
| `infra/server/` | Server deployment Compose template (rendered into `dist/` by `export-image`) |
| `plans/` | AI-generated plans (git-ignored) |
| `.agents/` | AI framework — repo-specific context files |

## Tech stack

| Concern | Technology |
|---|---|
| Language | Go (go.mod 1.25, built with the current stable toolchain) |
| AWS | AWS SDK for Go v2 (`service/route53`) |
| HTTP | standard library `net/http` |
| Logging | `log/slog`, structured JSON on stdout |
| Scheduling | `time.Ticker` — no cron |
| Tests | `testing` + `testify`, `go test -race` |
| Container | multi-stage Dockerfile → `scratch`, no ports; no `USER` baked in — the engine/deployment manages the user |

---

## Prerequisites

**Development machine**
- Go ≥ 1.25, `make`
- Docker or Podman (every recipe auto-detects Podman first, then Docker)

**AWS**
- A Route 53 **hosted zone** for your domain, and the zone id
- Permission to create an IAM policy + user (one-time setup below)

**Deployment target (server)**
- Any Docker-capable engine (`docker` or `podman`) on `linux/amd64`
- One writable folder for the app's data (see Deployment)

---

## Configuration

### The app data folder

Everything the app reads and writes locally lives in **one folder**: the INI
configuration file `aws-ddns.ini` (optional) and the last-known-IP cache `last-ip.txt`.
Resolution order: `-data-dir <path>` flag → `DATA_DIR` environment variable → default
`/var/lib/aws-ddns`. At startup the folder is created when absent and probed for
read/write; a permission problem exits immediately (code 2) with a clear message.
`DATA_DIR` can never be set inside the INI — the file lives in the folder it would name.

### Sources and precedence

**defaults < INI file < environment variables**

- **Production (server):** put `aws-ddns.ini` in the data folder. Template:
  `apps/aws-ddns/aws-ddns.ini.example` — copy it to `aws-ddns.ini` (git-ignored: it holds
  credentials) and fill it in. Keys are the same names as the variables below; `;`/`#`
  comments and cosmetic `[section]` headers are allowed; unknown keys are rejected at
  startup so a typo fails loudly.
- **Local development:** environment variables via the root `.env` (`make init-env`,
  then fill in values). Any variable set in the environment overrides the file.

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

IP discovery endpoints are fixed (ipify, then checkip). Empty optional variables fall
back to their defaults, so a blank `.env` line is safe.

---

## AWS setup — dedicated identity, least privilege

1. In `apps/aws-ddns/iam-policy.json`, replace `HOSTED_ZONE_ID` with your zone id and
   `RECORD_NAME` with the record name in **lowercase** (Route 53 normalizes names).
2. Create a customer-managed policy from that file, a dedicated IAM user (e.g.
   `aws-ddns`) with **no console access**, attach the policy, create one access key.
3. Put the key in your `aws-ddns.ini` (prod) or `.env` (local).

The policy allows exactly: `UPSERT` of that one `A` record (conditioned on record name,
type `A`, action `UPSERT`), plus zone-scoped `route53:ListResourceRecordSets` so the
daemon can read the record before deciding to write. It cannot create/delete zones or
touch any other record.

---

## Build and development

All commands run from the repository root.

| Target | Description |
|---|---|
| `init-env` | Create or sync `.env` from `.env.example` |
| `install` | Download Go module dependencies |
| `build` | Build the static binary (`apps/aws-ddns/bin/aws-ddns`) |
| `clean` | Remove build artefacts |
| `test` | Unit tests (`go test -race`) |
| `format` / `lint` | `gofmt` / `go vet` |
| `check` | **Full quality gate**: clean → install → format → lint → build → test |
| `start` / `update` | (Re)build and (re)create the daemon container via Compose |
| `shutdown` | Stop the container |
| `prune` / `teardown` | Remove project containers/images (+ volumes for `teardown`) |
| `export-image` | Build + pack the deployment image per architecture and render the server Compose file, all into `dist/` (see Deployment) |

App-level extras (run in `apps/aws-ddns/`): `make image` (container image, native
architecture) and `make image-amd64 VERSION=…` (`linux/amd64`, without packing).

Run `make check` before committing — see `AGENTS.md` for the full Definition of Done.
This repository intentionally has **no e2e app and no database** (`AGENTS.md` →
"Intentional deviations").

---

## Local testing

The machine must reach the Internet through the **same NAT** as the target server (no VPN, no
alternate gateway), or the discovered IP will not be the server's. Test against a temporary
record such as `ddns-test.example.com` first.

```bash
# 1. Directly on the machine (uses your shell env or root .env values).
#    Point the data folder somewhere writable — /var/lib/aws-ddns needs root:
export $(grep -v '^#' .env | xargs)          # or set the variables by hand
cd apps/aws-ddns
DATA_DIR=./data RECORD_NAME=ddns-test.example.com go run ./cmd/aws-ddns

# 2. In a container (from the repo root, uses root .env):
make start                                   # follow logs: podman compose ... logs -f aws-ddns

# 3. Verify against the zone's authoritative servers (bypasses caches):
dig +short ddns-test.example.com @$(dig +short NS example.com | head -1)
curl -s https://api.ipify.org                # should print the same address
```

`aws-ddns -version` prints the stamped version; startup logs the record, interval, TTL,
and data folder (never credentials).

---

## Deployment

**This version targets a local, registry-less build**: the image is built on your own
machine and imported manually on the target — no container registry involved. It is
suitable for local servers (any Docker-capable engine).

### 1. Build, pack, export — no registry

```bash
make export-image
# → dist/aws-ddns-<VERSION>-linux-amd64.tar.gz  (+ .sha256)
# → dist/aws-ddns-<VERSION>-linux-arm64.tar.gz  (+ .sha256)
# The tag comes from the root VERSION file (see Operations → Versioning).
```

Builds the image for **both supported architectures** (cross-compiled natively — no
emulation) and packs each for manual transfer. **Pick the archive matching the target's
CPU**: `uname -m` on the target → `x86_64` means `amd64`, `aarch64` means `arm64`. (A
wrong-arch image imports without complaint and then dies instantly with **no logs** —
see Troubleshooting.)

Copy the right archive + its `.sha256` to the target (scp, share, USB — anything), then:

```bash
sha256sum -c aws-ddns-<VERSION>-linux-<arch>.tar.gz.sha256   # verify the copy
docker load -i aws-ddns-<VERSION>-linux-<arch>.tar.gz         # or: podman load -i ...
docker run --rm localhost/aws-ddns:<VERSION> -version         # must print the version —
                                                              # proves the arch matches
```

Many container platforms also offer an image-import page that accepts the archive directly.

### 2. Prepare the data folder

Create one folder on the target (e.g. `/srv/aws-ddns`) and put
your filled-in `aws-ddns.ini` in it. The image bakes in no user id — the container
engine manages the user — so no ownership preparation is normally needed. The app still
verifies the folder at startup and logs `data directory … is not writable` (exit 2)
rather than failing silently if the platform runs the container as a user that cannot
write it.

### 3. Create the container

Two equivalent paths — use whichever your platform supports best:

- **Container-creation UI.** Create a container from the imported
  `localhost/aws-ddns` image and map the data folder to `/var/lib/aws-ddns` with the
  UI's **read/write permission** option on the mount. Set restart to
  `unless-stopped`/`always`. Nothing else: no ports, no environment variables, no
  command arguments. On some platforms this flow grants the mount permissions more
  reliably than their Compose flow — prefer it if the compose project logs
  `data directory … is not writable`.
- **Compose project.** `make export-image` renders **`dist/docker-compose.yaml`** — a
  self-contained deployment file (no `.env`, no build context, no registry) pinning the
  exact image version and carrying the whole runtime definition (`restart`, `read_only`,
  `cap_drop: ALL`, no ports). Paste it into your platform's compose/project editor and
  edit the one marked line — the host data-folder path. The template lives at
  `infra/server/docker-compose.yaml`.

### Upgrading — recreate, never "update"/pull

A registry-less deployment has nothing to pull from, so **a platform's
"update container" / "pull latest" action can never work here** — it attempts a
registry pull of `localhost/aws-ddns:<tag>` and fails (typically with
`unsupported manifest media type`, `pull access denied`, or `manifest unknown`). The
failure is harmless — the running container is untouched — but it is not the upgrade
path. The upgrade path is:

1. Copy and `docker load` the new version's archive (as in step 1).
2. **Recreate the container** pointing at the new `localhost/aws-ddns:<version>` tag:
   delete the old container and create it again (UI), or replace the compose project's
   content with the newly rendered `dist/docker-compose.yaml` and re-deploy it, or on
   the CLI: `docker rm -f aws-ddns` then the `docker run` command with the new tag.
   Recreating loses nothing — the INI and the state file live in the host data folder.
3. Optionally remove the old image: `docker rmi localhost/aws-ddns:<old-version>`.

<details>
<summary>Details: how the container finds the INI and state file</summary>

The app finds both through the data folder. Three ways to name it — you almost always
need only the first:

1. **Volume mount alone (standard).** Inside the container the app already looks in
   `/var/lib/aws-ddns` (the default). Map your host folder there.
2. **`DATA_DIR` env variable** — only for a different path *inside* the container: set
   `DATA_DIR=/data` and mount the volume at `/data`; the two must match — `DATA_DIR`
   names where the app looks, the volume decides what is actually there.
3. **`-data-dir` command argument** — equivalent (the flag wins over the env), but UIs
   usually bury the command override; prefer the env variable.

CLI equivalent:

```bash
docker run -d --name aws-ddns --restart unless-stopped \
  --read-only --cap-drop ALL \
  -v /path/on/host/aws-ddns:/var/lib/aws-ddns \
  localhost/aws-ddns:<VERSION>
```

</details>

No ports are published — the daemon makes outbound HTTPS calls only.

### 4. Verify

- Container logs show `starting aws-ddns` then `record synchronized` (or
  `record already up to date`).
- `dig +short home.example.com @<authoritative NS>` returns the network's public IP.

---

## Operations

- **Logging:** structured JSON, **everything on stdout** — the container's default log
  sink. That is exactly what Docker's logging driver captures and what **any Docker log
  view — a container platform's "Logs" view, a container management UI, `docker logs` — displays**;
  no log file, mount, or driver configuration is needed. This
  includes every startup failure (bad configuration, unusable data folder): a crash
  always leaves a log entry there, never a silent exit. Entries are written unbuffered,
  one JSON line each. Level via `LOG_LEVEL` (`verbose|debug|info|warning|error`).
  Credentials are never logged.
- **Startup trail:** the app logs each startup step in order — `starting aws-ddns`
  (version, os/arch, pid) → `resolving app data folder` (path + which source won:
  flag/environment/default) → `app data folder ready` → `configuration loaded` (INI
  found or not, record, interval, TTL, level) → `AWS configuration loaded` (region,
  credentials source) → `synchronization loop starting`. When something fails, the
  **last line tells you exactly which step died**.
- **Cycle logs:** each cycle carries a `cycle` number, start/completion entries with
  `duration` and `nextCheckIn`, and failures log `nextRetryIn`. At `LOG_LEVEL=debug`
  every internal step is logged too: each discovery endpoint queried (with duration),
  the cache comparison, the Route 53 read (value found, duration), and the upsert. A
  panic inside a cycle is caught, logged with its stack, and the loop keeps running;
  a panic anywhere else is logged with its stack before the process exits.
- **Exit codes:** `0` graceful shutdown or `-version`; `2` invalid configuration or an
  unusable data folder; `1` AWS configuration failure at startup. Runtime sync failures
  never exit — logged and retried next cycle.
- **Versioning:** the root `VERSION` file (`<Major>.<Minor>.<Patch>`) is the source of
  truth — the Makefiles stamp it into the binary and use it as the image tag; every
  change request bumps `Patch`, `Major`/`Minor` only on explicit request (`AGENTS.md`
  → Versioning). `docker run --rm localhost/aws-ddns:<tag> -version` prints what an image holds.
- **Troubleshooting:**
  - **Container exits with no logs at all (empty container Logs view)** → the
    process itself never ran; nothing that runs can be silent, because the very first
    instruction logs `starting aws-ddns`. In order of likelihood:
    1. **Wrong architecture.** A mismatched image imports without complaint and dies
       instantly with `exec format error`. On the target: `uname -m`
       (`x86_64` → load the `amd64` archive, `aarch64` → the `arm64` one) and compare
       with `docker image inspect localhost/aws-ddns:<tag> --format '{{.Architecture}}'`.
       The one-shot proof either way: `docker run --rm localhost/aws-ddns:<tag> -version` — if it
       prints the version, the architecture is fine.
    2. **An old image is still deployed.** `docker run --rm localhost/aws-ddns:<tag> -version`
       must print the current `VERSION`; versions before 1.0.1 wrote early failures to
       stderr only, which some log views drop.
    3. **The log view only streams live.** A crash-looping container may show a blank
       "real-time" log view between restarts — check
       `docker inspect aws-ddns --format '{{.State.ExitCode}} {{.State.Error}} {{.RestartCount}}'`
       and read the full capture with `docker logs aws-ddns` over SSH (or the log view's
       export button).
  - **`PUID`/`UID`/`USER_ID`/`GID`/`PGID`/`GROUP_ID` variables** some container platforms inject
    are a convention of certain image families — **this image ignores them**. No user
    id is baked into the image or the compose file: the container engine manages the
    user, so the data folder normally needs no ownership preparation at all.
  - `data directory ... is not writable` on first start → your platform runs the
    container as a specific non-root user that cannot write the mounted host folder.
    Either grant that user write on the folder (host folder permissions), or pin the
    owner explicitly with a `user: "<uid>:<gid>"` line in the compose project.
  - `unknown key "..."` → typo in `aws-ddns.ini`; keys use the exact names from the
    Settings table.
  - Sync failures → set `LOG_LEVEL=debug` (INI or env) and read the per-step entries to
    see which call fails (which discovery endpoint, the Route 53 read, or the upsert)
    and how long it took.
  - Record edited externally but IP unchanged → delete `last-ip.txt` in the data folder to
    force a Route 53 comparison on the next cycle.
  - Discovered IP looks wrong locally → check the machine isn't on a VPN or a different
    gateway than the server.

---

## Contributing

- Read `AGENTS.md` first — it is the single source of truth for rules, boundaries, and
  the Definition of Done.
- Keep `docs/architecture.md` and this README up to date with any structural change.
- Run `make check` before committing.
