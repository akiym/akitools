---
name: just
description: Task runner for this repo's Go test/build/lint/fmt. Invoke as `just --justfile .claude/skills/just/justfile <recipe> [args]`. Recipes pre-seed the sandbox-friendly Go and golangci-lint cache paths, so no env prefix is needed.
allowed-tools: Bash(just --justfile .claude/skills/just/justfile:*)
---

| recipe | command                  | default args |
|--------|--------------------------|--------------|
| test   | `go test -race "$@"`     | `./...`      |
| build  | `go build "$@"`          | `./...`      |
| lint   | `golangci-lint run "$@"` | *(none)*     |
| fmt    | `golangci-lint fmt`      | —            |

Extra args after the recipe name are forwarded via `"$@"` (safely quoted).