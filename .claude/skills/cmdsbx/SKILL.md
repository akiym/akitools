---
name: cmdsbx
description: Run a command inside a disposable Docker sandbox via the `cmdsbx` CLI. Use when a command is not permitted on the host (interpreters like python/node/awk, unfamiliar CLIs, untrusted or generated code) and it does not need to read or write host files — only stdout/stderr output matters. Examples - compute something with a python one-liner, test a code snippet, try an unfamiliar tool safely.
allowed-tools: Bash(cmdsbx do:*)
---

# cmdsbx

Run host-restricted or untrusted commands inside a throwaway Docker container.
The container has no network, sees no host files, and is removed after the
command exits. Exit code, stdout, and stderr propagate to the caller.

## When to use

- The command is an interpreter or tool not allowed on the host (python, node, awk, ruby, perl, ...)
- Running untrusted or freshly generated code snippets
- Computing or verifying something where only the printed output matters

## When NOT to use

- The command must read or write host files, or needs network access — see
  "Host access" below
- Interactive/TTY programs

## Usage

`cmdsbx do` accepts only `--image`, `--env KEY=VALUE`, `--workdir`, and
`-i` (stream stdin in, like docker run -i). There is no way to mount host
paths or enable network from `do` — such flags are parse errors by design.

```bash
# bash snippet on the default image (ubuntu:24.04), no network, no host files
cmdsbx do -- bash -c 'seq 1 3 | while read i; do echo "line $i"; done'

# python one-liner
cmdsbx do --image python:3.14-slim -- python3 -c 'print(1 + 2)'

# node
cmdsbx do --image node:24-slim -- node -e 'console.log(6 * 7)'

# awk
echo "1\n2" | cmdsbx do -i -- awk '{ print $1 }'

# pass code via stdin (-i required) to avoid shell-quoting issues
echo 'print("hi")' | cmdsbx do -i --image python:3.14-slim -- python3 -

# multi-line snippets: heredoc into stdin
cmdsbx do -i --image python:3.14-slim -- python3 - <<'EOF'
import json
print(json.dumps({"ok": True}))
EOF
```

Pass input data via stdin (requires `-i`) or `--env`; collect results
from stdout.

## Host access requires `cmdsbx unsafe`

Reading host files, writing to the host, or network egress is the job of
`cmdsbx unsafe` (e.g. `cmdsbx unsafe --rootfs "$PWD" -- go test ./...`).
It is intentionally NOT covered by this skill's allowed tools: using it
prompts the user for approval. Only reach for it when the task genuinely
needs host files or network, say why in one sentence before running it, and
never ask the user to allow `cmdsbx unsafe` unconditionally.

## Broker

In sandboxed agent environments without Docker socket access, `cmdsbx do`
transparently delegates the run to a `cmdsbx broker` daemon over a unix
socket; usage is identical. If `do` reports the broker socket is not
reachable and docker fails with a socket permission error, the daemon is
not running — ask the user to start `cmdsbx broker` instead of trying
`cmdsbx unsafe` or other workarounds.

## Rules

- Pick the smallest official image that has the needed interpreter (`python:3.14-slim`, `node:24-slim`, default `ubuntu:24.04` for shell tools)
- If the command fails with "executable file not found", the image lacks the tool — switch images rather than installing packages
- Do not use `cmdsbx run`/`exec` sessions here; one-shot `cmdsbx do` keeps cleanup automatic
- `cmdsbx do` runs with `--pull=never`; if the image isn't already pulled it fails immediately. Ask the user to pull the image (e.g. `docker pull python:3.14-slim`) rather than auto-pulling arbitrary images from an agent context
