# AGENTS.md — aws-ddns

All rules from `AGENTS.md` (repository root), `.agents/context/architectures/layered.md`,
and `.agents/context/stacks/go.md` apply in full.
This file documents only what is specific to this app.

---

## App Role and Scope

The daemon that keeps one Route 53 `A` record synchronized with the network's current public
IPv4. It discovers the IP over external HTTPS endpoints (with fallback), reads the record
through the Route 53 API, and issues an `UPSERT` only when the value changed. It repeats at
`INTERVAL`, survives transient failures, shuts down gracefully, and exposes nothing inbound.

---

## App-specific Overrides

- **Entry layer is `process` only.** There is no `commands` surface; the only flags are
  `-version` and `-data-dir <path>`. Rationale: headless container daemon, recorded at
  scaffold time in root `AGENTS.md` → "Intentional deviations".
- **One app data folder** (root `AGENTS.md` → Rules): `-data-dir` > `DATA_DIR` >
  `/var/lib/aws-ddns`, holding `aws-ddns.ini` and `last-ip.txt`. It is created and probed
  read/write at startup (`framework.EnsureDataDir`) — fail fast with exit 2 on a permission
  problem; never scatter other local files outside it.
- **Configuration precedence is defaults < INI file < environment variables** (root
  `AGENTS.md` → Rules). The INI parser is the app's own minimal one
  (`internal/framework/config_file.go`) — do not introduce an INI library. `DATA_DIR` is
  rejected as an INI key by design.
- **The last-known-IP cache is best-effort.** `SyncService` skips Route 53 only on a cache
  hit; it never caches a failed upsert, and any state-file error degrades to the uncached
  path with a warning. Keep those invariants — they are what makes the cache safe.
- **Approved dependencies:** AWS SDK for Go v2 (Apache-2.0), `stretchr/testify` (MIT,
  tests only). Everything else is standard library.
- **IAM:** the dedicated identity uses `iam-policy.json` — conditioned `UPSERT` on the one
  record plus zone-scoped `route53:ListResourceRecordSets` for the read-before-write
  comparison. Never widen it.
- **Exit codes:** `0` on graceful shutdown (signal) and for `-version`; `2` on invalid
  configuration or an unusable data folder; `1` when AWS configuration cannot be loaded at
  startup. Runtime sync failures never exit — they are logged and retried on the next cycle.
- **Deployment target:** `linux/amd64` container image (`make image-amd64`); local runs may
  use the native architecture (`make image`, or `go run ./cmd/aws-ddns`).
