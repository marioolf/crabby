# Contributing to Crabby

Thanks for your interest in Crabby! It is intentionally small — please keep
changes focused and in line with its philosophy:

> Make working with multiple Claude Code sessions effortless.

Crabby is **not** a Git client, a tmux replacement, a project manager, an IDE,
or an AI orchestrator. Features outside that scope will not be accepted.

## Requirements

- [Go](https://go.dev) 1.23+
- tmux and [Claude Code](https://claude.com/claude-code) (to run it end-to-end)
- WSL2 + Ubuntu (the only supported runtime)

## Development

```bash
make build   # compile ./cmd/crabby into bin/
make test    # run the test suite
make fmt     # format the code
make lint    # go vet + gofmt check
```

Run `./bin/crabby doctor` to confirm your environment.

## Guidelines

- Prefer the standard library; avoid new dependencies unless truly necessary.
- Keep everything under `internal/`; there is no public API.
- Match the style and comment density of the surrounding code.
- Run `make lint test` before opening a pull request.
- Add a note under the `## [Unreleased]` section of `CHANGELOG.md`.

## Releasing

Releases are automated — see [`docs/RELEASING.md`](docs/RELEASING.md).
