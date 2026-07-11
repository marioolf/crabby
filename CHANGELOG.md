# Changelog

All notable changes to Crabby are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.3.1] - 2026-07-11

### Added

- Remove a project from Crabby without deleting any files: press **d** on the
  home screen (confirmed with `y/n`), or run `crabby rm [project]` (`-y` to skip
  the prompt). A running session is stopped first; your `CLAUDE.md` and
  `.claude/` are left untouched, so `crabby init` can re-add it later.

## [0.3.0] - 2026-07-11

### Added

- **Local project packs.** `crabby init` can apply a reusable pack — just a
  directory of files copied into the project. Packs live in
  `~/.config/crabby/packs/<name>/` and carry a small `pack.yaml` manifest.
  - One pack → used automatically; several packs → a Bubble Tea selector.
  - Existing project files are never overwritten (they're reported and kept).
  - No variables, scripting, dependencies, or remote sources — only folders.
  - An example lives in `examples/simple-pack/`; copy it into your packs dir to
    start your own.
- **Crab-army header** on the home screen (Lip Gloss), giving Crabby a playful,
  recognizable identity.
- **Stop a session from the home screen** with `x` (confirmed with `y/n`) — no
  need to attach or exit Claude.
- **New project** from the home screen with `n`, which initializes the current
  directory (with pack selection).

### Changed

- The home screen shows the new header and an updated key bar
  (`enter open · n new · x stop · r refresh · q quit`).
- README documents packs: what they are, where they live, and how to create,
  use, and customize one.

### Removed

- The auto-generated `completion` command is hidden to keep the CLI minimal.

## [0.2.0] - 2026-07-11

### Changed

- `crabby` with no arguments now opens the home screen (the TUI). `crabby ps`
  remains as an alias.
- The home screen is a loop: pick a project → work in Claude → leave → you land
  back on the list automatically, ready to pick another. No shell commands, no
  session names to remember.
- Selecting a project starts its session if it isn't running yet, so opening a
  project is always a single keypress.
- The TUI shows more, minimally: project name, session state (with clearer
  colors), path, git branch, and time since last activity.
- README reframed around "a workspace manager for Claude Code"; tmux is
  described only as an implementation detail.

### Added

- Leave a session and return to Crabby with a single key press (**F12** by
  default) — no tmux shortcuts. The key is configurable via `detach_key` in
  `~/.config/crabby/config.yaml`.
- Sessions show a small Crabby-branded status bar with the current project and a
  reminder of the return key, so tmux stays out of sight.
- Crabby runs on its own dedicated tmux server (socket `crabby`), isolated from
  any tmux the user runs themselves.

## [0.1.1] - 2026-07-11

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
  WSL": it converts the working directory with `wslpath` inside the distro
  instead of via a fragile separate `wsl wslpath` invocation.
- Windows wrapper no longer drops command arguments (`crabby doctor` ran as
  bare `crabby`). The forwarded command is now base64-encoded so `wsl.exe`
  cannot mangle its quoting, and runs via `bash <(…)` so `crabby ps` and
  `crabby attach` keep a real terminal on stdin.

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

[Unreleased]: https://github.com/marioolf/crabby/compare/v0.3.1...HEAD
[0.3.1]: https://github.com/marioolf/crabby/compare/v0.3.0...v0.3.1
[0.3.0]: https://github.com/marioolf/crabby/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/marioolf/crabby/compare/v0.1.1...v0.2.0
[0.1.1]: https://github.com/marioolf/crabby/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/marioolf/crabby/releases/tag/v0.1.0
