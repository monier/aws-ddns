# aws-ddns (app)

The Dynamic DNS daemon: keeps one AWS Route 53 `A` record pointed at the network's
current public IPv4, querying Route 53 only when the address actually changed.

**All documentation lives in the [root README](../../README.md)** — context, how it
works, prerequisites, configuration (data folder, INI, env), AWS/IAM setup, build,
local testing, deployment, and operations. This file stays minimal on purpose
(root `AGENTS.md` → "Intentional deviations").

## Code map

| Path | Contents |
|---|---|
| `cmd/aws-ddns/` | Composition root (`main.go`), `-version` and `-data-dir` flags |
| `internal/process/` | Daemon loop: startup run, ticker, graceful shutdown, cycle logging |
| `internal/services/` | `SyncService` (discover → cache check → compare → upsert) and its collaborator interfaces |
| `internal/repositories/` | HTTPS IP discoverer (fallback, validation), the Route 53 adapter, and the file state store |
| `internal/framework/` | App data folder resolution/probe, configuration (INI + env merge), the slog JSON logger |

Supporting files: `Dockerfile` (multi-stage → `scratch`; no baked-in user — the engine
manages it), `iam-policy.json`
(least-privilege template), `aws-ddns.ini.example` (production config template).

## App-level Make targets

`install` · `build` · `clean` · `test` · `format` · `lint` · `image` (native arch) ·
`image-amd64 VERSION=…` (`linux/amd64`). Day-to-day work goes through the root Makefile
(`make check`, `make export-image`).

Rules and app-specific overrides: `AGENTS.md` here and at the root.
