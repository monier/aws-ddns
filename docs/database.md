# Database Models

This repository has **no database**. The only local state `aws-ddns` keeps is a single
plain-text file caching the last synchronized IP (`<data-dir>/last-ip.txt` — see
`docs/architecture.md`); everything else is re-derived from the discovery endpoints and
Route 53 itself.

This file exists to satisfy the canonical repository structure; add an `erDiagram` per
database here if a database is ever introduced (none is planned — see `AGENTS.md`).
