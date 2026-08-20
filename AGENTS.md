# AGENTS.md

This file is the single source of truth for this repository.
All AI tools, agents, and contributors must read and follow these rules.
No other file may restate, reinterpret, or extend what is written here.

---

## Repository Purpose

`aws-ddns` is a small Dynamic DNS service that keeps an AWS Route 53 `A` record synchronized
with the current public IPv4 address of the network where it runs. It targets a Docker-capable
local server behind a residential router with a dynamic public IPv4, discovers the public IP through
external HTTPS endpoints (never a router API), and updates the record with a Route 53 `UPSERT`
only when the value changed.

---

## Repository Structure

```
apps/
  aws-ddns/     # The single app — a Go daemon (Layered, no CQRS, entry layer `process`)
.agents/
  context/      # Repo-specific knowledge bases, grouped by category
docs/           # Architecture (and database) documentation
infra/local/    # Local Docker Compose environment
infra/server/      # Server deployment Compose template (version rendered in by `make export-image`)
plans/          # AI-generated plans (git-ignored, never committed)
AGENTS.md       # This file — root rules
CLAUDE.md       # Claude Code adapter (pointer only)
README.md       # Human-readable introduction
Makefile        # Root recipes (delegate to apps/aws-ddns)
```

---

## Rules

- The app is a **headless daemon**: it exposes no inbound network port, serves no HTTP, and
  has no interactive command surface beyond a `-version` flag.
- The app owns one **configurable app data folder** (`-data-dir` flag, then the `DATA_DIR`
  environment variable, default `/var/lib/aws-ddns`) where it must have read/write
  permission — verified at startup (created if absent; a clear error otherwise). Everything
  the app reads and writes locally lives there: the INI configuration file
  (`aws-ddns.ini`) and the last-known-IP state file (`last-ip.txt`). Because the INI lives
  inside the folder, the folder's location is flag/env-only, never an INI key.
- Configuration has two sources, merged as **defaults < INI file < environment variables**:
  local development uses environment variables (root `.env`, git-ignored); production uses
  `<data-dir>/aws-ddns.ini` (template `apps/aws-ddns/aws-ddns.ini.example`). Secrets are
  never embedded in code, the binary, or the Docker image, and never committed —
  `aws-ddns.ini` is git-ignored.
- The last synchronized IP is cached in the state file, so Route 53 is queried only when
  the discovered address changed (or the cache is empty/unreadable). The cache must never
  be written on a failed upsert, and runtime state-file failures must degrade to querying
  Route 53 — never crash the daemon.
- AWS access uses a dedicated IAM identity restricted by `apps/aws-ddns/iam-policy.json`:
  `UPSERT` of the one configured `A` record, plus zone-scoped read
  (`route53:ListResourceRecordSets`) needed to compare before writing. Nothing broader.
- The service must keep running through transient discovery or AWS failures, and must shut
  down gracefully on `SIGTERM`/`SIGINT`.
- Keep the project intentionally small: no frameworks, no database, no scheduler or message
  queue, no web server. New dependencies require explicit approval recorded here.
- Architecture and stack conventions live in `.agents/context/` (see Context below) — follow
  them; do not restate them elsewhere.

---

## Intentional deviations from the ai-assist canonical structure

Recorded by developer decision at scaffold time (2026-08-20):

- **No e2e app**: `e2e/`, `infra/local/docker-compose.e2e.yaml`, and the `e2e-run`/`e2e-clean`
  targets are intentionally absent. Validation is unit tests plus the documented manual
  scenario (`dig` against a test record — see `apps/aws-ddns/README.md`).
- **No database**: the `db-*` targets are absent; `docs/database.md` records that no database
  exists.
- **Narrowed heavy-client contract**: no CLI command surface, no multi-OS packaging matrix,
  no OS-native log sink, no single-instance lock. The app is a container daemon with one
  deployment target (`linux/amd64`); the container runtime owns instance lifecycle and log
  collection (structured JSON on stdout).
- **Documentation placement**: the **root `README.md` is the single comprehensive
  document** (context, prerequisites, configuration, build, testing, deployment,
  operations); `apps/aws-ddns/README.md` is intentionally minimal — a pointer plus the
  code map and app-level targets. Detailed user/operator documentation belongs in the
  root README, never duplicated in the app README.

---

## Versioning

The repository version lives in the root `VERSION` file, format
`<Major>.<Minor>.<Patch>`. The Makefiles read it as the default: it is stamped into the
binary (`-version`) and used as the container image tag.

- **Every change request increments `Patch`** — bumping `VERSION` is part of delivering
  any change.
- **`Major` and `Minor` are incremented only on the developer's explicit request** —
  never on an agent's or contributor's own initiative.

---

## DO NOT

- Do not add an inbound port, HTTP server, or any router/ISP-specific integration.
- Do not widen the IAM policy beyond the configured hosted zone and record.
- Do not log credentials, tokens, or the AWS secret key — mask anything sensitive.
- Do not commit secrets or `.env`; it is git-ignored.
- Do not add a dependency without approval recorded in this file or the app's `AGENTS.md`.
- Do not leave `README.md` or `docs/architecture.md` stale when structure or behavior changes.

---

## Definition of Done

A unit of work is complete when all of the following are true:

1. `make check` passes (clean → install → format → lint → build → test).
2. Unit tests cover the change, including error paths (`go test -race`).
3. No secrets are introduced; nothing sensitive is logged.
4. `README.md`, `docs/architecture.md`, and the app docs are updated if the change affects
   what they describe.
5. Dependencies carry approved licences (see the app `AGENTS.md` approved list).
6. The `VERSION` file is incremented per the Versioning rules above.

---

## Commands

All commands are invoked through Make — see the root `Makefile` and `README.md` for the
target table. App-level targets live in `apps/aws-ddns/Makefile`.

---

## Context

`.agents/context/` contains the repo-specific knowledge bases:

| File | Contents |
|---|---|
| `architectures/layered.md` | The Layered architecture as realised in this repository (entry layer `process`, `services`, `repositories`, `framework`) |
| `stacks/go.md` | Go stack conventions: toolchain, allowed dependencies, logging, testing, static build |
| `standards/git-workflow.md` | Git workflow: trunk-based on `main`, Conventional Commits, push policy |

---

## Scoping

- This file applies to the entire repository.
- `apps/aws-ddns/AGENTS.md` may extend these rules for that app only.
- The closest `AGENTS.md` to the code always takes precedence.
- No other file participates in rule scoping or overrides.
