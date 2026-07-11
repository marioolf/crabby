# Changelog

All notable changes to Crabby are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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
