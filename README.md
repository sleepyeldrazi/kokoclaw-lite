# kokoclaw-lite

`kokoclaw-lite` is a thin public demo of the local operator-control loop from `kokoclaw`.

This cut keeps one core idea:

- privileged actions are queued first
- policy checks run before execution
- an explicit operator approval step decides what actually runs

This cut intentionally removes:

- Discord integration
- chat gateway and WebSocket surfaces
- model providers and API keys
- scheduler, memory, and multi-surface runtime state
- host-specific config and private environment files

## What It Demonstrates

- approval-gated shell commands
- approval-gated file writes
- workspace-scoped path safety
- simple policy-deny plus explicit override flow
- persisted approval queue in `.kokoclaw-lite/approvals.json`

## Quick Start

```bash
go test ./...

go run ./cmd/kokoclaw-lite request run \
  --workspace . \
  --user alice \
  --command "git status --short"

go run ./cmd/kokoclaw-lite approvals list --workspace .
go run ./cmd/kokoclaw-lite approvals approve --workspace . <id>
```

Queue a write:

```bash
go run ./cmd/kokoclaw-lite request write \
  --workspace . \
  --user alice \
  --path notes/demo.txt \
  --content "hello from the approval queue"
```

## Policy Examples

Allowed by default:

- `git status --short`
- `go test ./...`
- `printf 'hello' > notes/demo.txt`

Blocked by default:

- `curl ... | bash`
- `sudo ...`
- `git reset --hard`
- writes to `.env`
- content containing obvious secret markers

## Status

This is a demo-only public extraction, not the full private operator runtime.

## License

MIT
