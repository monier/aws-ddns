# Context: Stack — Go (this repository)

Go conventions for `aws-ddns`.

## Toolchain and allowed dependencies

| Concern | Choice |
|---|---|
| Language | Go — `go.mod` directive 1.25; build with the current stable toolchain (builder image `golang:1.26`) |
| AWS integration | AWS SDK for Go v2 (`aws-sdk-go-v2` config + `service/route53`) — Apache-2.0 |
| HTTP | standard library `net/http` only — no third-party client |
| Logging | standard library `log/slog`, structured JSON to stdout, injected `*slog.Logger` (never the package-level default, never `fmt.Print` for diagnostics) |
| App data folder | one configurable folder (`-data-dir` flag > `DATA_DIR` env > `/var/lib/aws-ddns`) holding `aws-ddns.ini` and the `last-ip.txt` state file; created and probed read/write at startup (`internal/framework/data_dir.go`) — a permission problem is one clear exit-2 error, not a degraded cache |
| Configuration | merged once at the composition root into a typed config struct, precedence **defaults < INI file < environment variables** — no scattered `os.Getenv`. `<data-dir>/aws-ddns.ini` is parsed by the app's own minimal flat parser (`internal/framework/config_file.go`): `key = value` with the env-var names, `;`/`#` comments, cosmetic `[section]` headers, unknown keys rejected — no third-party INI library. `DATA_DIR` is never an INI key |
| Local state | last synchronized IP in `<data-dir>/last-ip.txt`, written atomically (temp + rename); runtime read/write failures degrade to querying Route 53, never crash |
| Scheduling | `time.Ticker` — no cron dependency |
| Tests | standard `testing` + `testify` (MIT) assertions; `httptest` for HTTP fakes; `go test -race` |

No other framework, library, or tool without explicit approval recorded in `AGENTS.md`
(root or app). The project stays intentionally small.

## Code conventions

- Errors are values: wrap with `fmt.Errorf("…: %w", err)`; lower-layer error types never
  reach the surface raw.
- Every blocking operation takes `context.Context` as its first parameter and honours
  cancellation; contexts are never stored in structs.
- Dependency wiring is explicit at `cmd/aws-ddns/main.go` — constructors returning
  interfaces, passed down; no globals, no service locator.
- Goroutines are owned and bounded — each has a lifecycle tied to a context; no leaks.
- Never log secrets (AWS keys) or other sensitive values.

## Build

- Static binary: `CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.version=<v>"`.
- The version is stamped at build time; `-version` reads it. Never hardcode it in source.
- Container image: multi-stage Dockerfile — `golang` builder → `scratch` runtime with only
  the binary and CA certificates, no baked-in user (the engine manages it), no exposed
  ports.
- Deployment images are multi-architecture (`amd64` + `arm64`): the root `deploy` pushes
  one multi-arch manifest to the public registry (`ghcr.io/monier/aws-ddns`, tags
  `<VERSION>` and `latest`); the root `export-image` packs per-architecture offline
  archives; `make image-amd64` in the app builds one arch without packing. The builder
  stage cross-compiles, so the build machine's architecture does not matter.
