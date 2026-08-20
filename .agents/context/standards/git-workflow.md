# Context: Git Workflow (this repository)

- **Trunk-based on `main`.** The default branch is `main`; day-to-day work is committed
  directly to it. A feature branch (`<type>/<short-description>`, kebab-case, `<type>` ∈
  `feat`/`fix`/`chore`) is used only when the developer explicitly asks for one.
- **Conventional Commits**, single-line messages: `<type>: <description>` with `<type>` ∈
  `feat`, `fix`, `chore`, `refactor`, `docs`, `test`, `ci`, `perf`. The description states
  what the change enables or fixes — not which files were touched. No ticket system is used
  for this repository.
- **Push policy:** the developer has opted into automatic push — push `main` to `origin`
  after each validated commit.
- **Quality gate before commit:** `make check` must pass.
- Never commit secrets or `.env`; `plans/` is git-ignored and never committed.
