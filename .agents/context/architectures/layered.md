# Context: Layered Architecture (this repository)

How the Layered architecture is realised in `aws-ddns`. CQRS is off.

## Layers

```
apps/aws-ddns/
  cmd/aws-ddns/main.go   # composition root only — load config, wire dependencies, start the loop
  internal/
    process/             # entry layer — the supervised daemon loop (no business logic)
    services/            # business layer — the synchronization decision (discover → compare → upsert)
    repositories/        # data access to external systems — the IP discovery endpoints, Route 53
    framework/           # cross-cutting — configuration, logging setup
```

## Rules

- **`process` holds no business logic.** It owns timing (run at startup, then every
  `INTERVAL`), cancellation, and cycle logging, and delegates each cycle to exactly one
  `services` operation.
- **`services` owns the decision.** It talks to its collaborators only through interfaces it
  defines (`IPDiscoverer`, `DNSRepository`) and never imports an SDK or `net/http` directly.
- **`repositories` adapts external systems** — the HTTPS IP discovery endpoints and the
  Route 53 API — behind the `services` interfaces. SDK/driver error types never escape a
  repository raw: wrap with context (`fmt.Errorf("…: %w", err)`).
- **`framework` is cross-cutting only** (config parsing, logger construction). It never
  contains business rules.
- Dependencies point inward: `cmd` → `process`/`services`/`repositories`/`framework`;
  `process` → `services`; `repositories` implement `services` interfaces. Nothing under
  `internal/` imports `process`.
- Interfaces and implementations live in separate files; one public type per file.
