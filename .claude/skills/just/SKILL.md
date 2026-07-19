---
name: just
description: Task runner for this repo's Go workflow. Use whenever you need to run `go build`, `go test`, `go run`, or `golangci-lint` — the raw commands fail under the sandbox because their default cache paths are not writable, and this skill's recipes pre-seed sandbox-friendly GOCACHE/GOMODCACHE/GOLANGCI_LINT_CACHE. Invoke as `just --justfile .claude/skills/just/justfile <recipe> [args]`.
allowed-tools: Bash(just --justfile .claude/skills/just/justfile test:*), Bash(just --justfile .claude/skills/just/justfile build:*), Bash(just --justfile .claude/skills/just/justfile lint:*), Bash(just --justfile .claude/skills/just/justfile fmt:*), Bash(just --justfile .claude/skills/just/justfile clean)
---

| recipe | command                       | default args |
|--------|-------------------------------|--------------|
| test   | `go test -race "$@"`          | `./...`      |
| build  | `go build "$@"`               | `./...`      |
| run    | `go run "$@"`                 | *(required)* |
| lint   | `golangci-lint run "$@"`      | *(none)*     |
| fmt    | `golangci-lint fmt`           | *(none)*     |
| clean  | `go clean -modcache -cache`   | *(none)*     |

Extra args after the recipe name are forwarded via `"$@"` (safely quoted).

## Notes

- `run` executes arbitrary Go code, so it is intentionally NOT in this
  skill's allowed-tools — invoking it prompts the user for approval. That
  is the intended flow; never ask to allowlist it unconditionally.
- The module cache under `/tmp/claude-501/gomod` is kept in Go's default
  read-only mode. If it ever gets corrupted (e.g. `dir has been modified`
  errors, or misleading `no required module provides package X` for a
  package that IS in go.mod), run the `clean` recipe — `go clean` handles
  the read-only bits itself, so no `chmod` dance is needed. The next
  recipe run repopulates the cache.