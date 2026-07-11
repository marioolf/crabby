# Changelog

All notable changes to Crabby are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Fixed

- Windows installer now downloads release assets reliably behind corporate
  proxies and through GitHub's redirect to `release-assets.githubusercontent.com`:
  it forces TLS 1.2, prefers `curl.exe`, and falls back to `Invoke-WebRequest`
  with a browser User-Agent and default proxy credentials.
- Windows installer no longer closes the terminal window: all control flow is
  wrapped in a function using `return` instead of top-level `exit`, which
  terminates the whole host when run via `irm … | iex`.
- Windows installer detects the Ubuntu distribution reliably by requesting
  UTF-8 output from `wsl` (`WSL_UTF8=1`) and stripping stray NUL bytes, fixing
  a false "No Ubuntu distribution found" that aborted the install.
- Windows wrapper (`crabby.exe`) no longer fails with "cannot convert path to
  WSL": it forwards in a single `wsl bash -lc` call that converts the working
  directory with `wslpath` inside the distro and passes arguments positionally,
  removing the fragile separate `wsl wslpath` invocation and all shell-quoting.

## [0.1.0] - 2026-07-11

### Added

- `crabby init` — prepare a project for Claude Code (`.claude/crabby.yaml`, `CLAUDE.md`, registry entry).
- `crabby start` — create the tmux session, launch Claude, and attach (or attach if it already exists).
- `crabby ps` — Bubble Tea TUI listing every project and its session state, with Enter-to-attach.
- `crabby attach` — attach directly to a project's Claude session.
- `crabby doctor` — verify WSL, Ubuntu, tmux, Claude, and crabby are present.
- `crabby version` — print the Crabby version.
- Windows wrapper (`crabby.exe`) that forwards commands into WSL.
- One-command installers for Windows (`install.ps1`) and Linux/WSL (`install.sh`).
- GitHub Actions release workflow producing Linux (amd64/arm64) and Windows (amd64/arm64) assets.

[Unreleased]: https://github.com/marioolf/crabby/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/marioolf/crabby/releases/tag/v0.1.0
