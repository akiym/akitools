---
name: cmdsbx
description: Run a command inside a disposable Docker sandbox via the `cmdsbx` CLI. Use when a command is not permitted on the host (interpreters like python/node/awk, unfamiliar CLIs, untrusted or generated code) and it does not need to write host files or reach the network — only stdout/stderr output matters. Read-only access to the project directory is available via `--mount-cwd-ro`. Examples - compute something with a python one-liner, test a code snippet, run project tests read-only, try an unfamiliar tool safely.
allowed-tools: Bash(cmdsbx do:*)
---

# cmdsbx

Run host-restricted or untrusted commands inside a throwaway Docker container.
The container has no network, sees no host files (except via `--mount-cwd-ro`,
below), and is removed after the command exits. Exit code, stdout, and stderr
propagate to the caller.

## When to use `cmdsbx do`

- The command is an interpreter or tool not allowed on the host (python, node, awk, ruby, perl, ...)
- Running untrusted or freshly generated code snippets
- Computing or verifying something where only the printed output matters
- Read-only work over the project files (`--mount-cwd-ro`), e.g. running tests

## When NOT to use `cmdsbx do`

- Interactive/TTY programs (pagers, editors, prompts) — no supported invocation
- The command must write host files or reach the network — `do` cannot; see **cmdsbx unsafe** below

## Invocation

`cmdsbx do` accepts only these flags; anything else is a parse error by design.

- `--image IMAGE`     container image (default `ubuntu:24.04`)
- `--env KEY=VALUE`   set an environment variable; repeat for multiple
- `--workdir DIR`     working directory inside the container
- `--mount-cwd-ro`    mount cwd read-only at the same path (see below)
- `-i`                stream stdin into the container (needed for stdin-piped commands and heredocs)

```bash
# bash snippet on the default image, no network, no host files
cmdsbx do -- bash -c 'seq 1 3 | while read i; do echo "line $i"; done'

# python one-liner
cmdsbx do --image python:3.14-slim -- python3 -c 'print(1 + 2)'

# node
cmdsbx do --image node:24-slim -- node -e 'console.log(6 * 7)'

# awk with piped stdin
echo "1\n2" | cmdsbx do -i -- awk '{ print $1 }'

# pass code via stdin (-i required) to avoid shell-quoting issues
echo 'print("hi")' | cmdsbx do -i --image python:3.14-slim -- python3 -

# multi-line snippet via heredoc (host shell handles the heredoc; -i required)
cmdsbx do -i --image python:3.14-slim -- python3 - <<'EOF'
import json
print(json.dumps({"ok": True}))
EOF

# pass data via --env (repeatable)
cmdsbx do --env NAME=world --env GREETING=hello -- bash -c 'echo "$GREETING $NAME"'
```

Pass input via stdin (with `-i`) or `--env`; collect results from stdout.

## Read-only project access: `--mount-cwd-ro`

When the command needs to READ project files (run tests, lint, inspect
sources), add `--mount-cwd-ro`: the current directory is mounted read-only
at the same path and becomes the default workdir.

```bash
cmdsbx do --mount-cwd-ro --image golang:1.26 -- go test ./...
```

Writes to the mount fail with a read-only error; that is by design.

## Host writes and network: `cmdsbx unsafe`

When the task genuinely needs to write host files or reach the network,
use `cmdsbx unsafe`. It is intentionally NOT in this skill's allowed-tools,
so running it prompts the user for approval — that's the correct flow, not
something to avoid.

```bash
# write back to the project (mount rw)
cmdsbx unsafe --rootfs "$PWD" --write -- go generate ./...

# network egress
cmdsbx unsafe --allow-egress --image alpine -- wget -qO- https://example.com
```

Rules for `unsafe`:

- Only reach for it when writes or network are actually required
- State in one sentence why before running so the approval prompt is informed
- Never ask the user to allowlist `cmdsbx unsafe` unconditionally

## When it fails

- `executable file not found` — the image lacks the tool. Switch to an image that has it; do not try to install packages at runtime.
- image missing / `--pull=never` refusal — ask the user to `docker pull IMAGE`.
- `--mount-cwd-ro` rejected — this environment does not permit read-only project mounts. If the task needs project reads, ask the user.
- sandbox not reachable / docker socket permission error — the runtime is not available; ask the user.

## Tips

- Pick the smallest official image that already has the needed interpreter (`python:3.14-slim`, `node:24-slim`, default `ubuntu:24.04` for shell tools).
- Only use `cmdsbx do` (and `unsafe` when justified) from this skill; `run`/`exec` sessions require manual cleanup and are out of scope.
