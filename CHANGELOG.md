# Changelog

All notable changes to Crabby are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.7.1] - 2026-07-13

### Changed

- Dashboard layout: everything is centred again, but the workspace list is
  centred as a single block so its name and data columns stay left-aligned
  (instead of each line drifting to its own centre). The banner, dividers, usage
  summary, and footer keys remain centred line by line.

## [0.7.0] - 2026-07-13

### Added

- **Multiple tasks per workspace.** A workspace can now hold several independent
  Claude sessions ("tasks") that share the same project directory — work on a
  refactor, tests, and docs in parallel without cloning the repo. New commands:
  - `crabby task create <name>` — start a new task and open it.
  - `crabby task list [workspace]` — list a workspace's tasks and their state.
  - `crabby task delete <name> [workspace]` — remove a task (the workspace stays).
  - `crabby task restart <name> [workspace]` — stop and relaunch a task.
- `crabby attach [workspace] [task]` now asks which task to open when a
  workspace has more than one; pass the task name to skip the prompt.
- The dashboard groups tasks beneath their workspace and adds a **t** key to
  start a new task in the selected workspace.

### Changed

- **Dashboard redesign.** The banner stays centred, but the workspace list and
  footer are now left-aligned like a normal CLI, with each workspace's data laid
  out in a column to the right of its name. Action keys in the footer are
  highlighted so they read distinctly from the informational usage summary.
- Session names for named tasks are generated automatically as
  `crabby_<workspace>_<task>`; the default task keeps the `crabby_<workspace>`
  name so single-task workspaces are unchanged.

### Notes

- Existing single-task workspaces migrate automatically: each becomes a
  workspace with one default "main" task on its original session. No action is
  required.
- With several tasks sharing one directory, transcript-derived detail (model,
  tokens, fine activity) is shown at the workspace level; per-task rows show the
  reliable tmux state, since Claude's transcripts can't be attributed to a
  specific tmux session.

## [0.6.0] - 2026-07-13

### Added

- **Workspace insights on the dashboard.** Each workspace now shows what Claude
  is doing (Thinking / Editing files / Reading files / Running command /
  Responding / Waiting for input / Idle), the model in use, tokens spent this
  session, uptime, and how long ago Claude last acted — all read from Claude
  Code's own session transcripts under `~/.claude/projects/`, not from terminal
  scraping.
- **Global usage summary** below the list: workspace counts by state, how many
  are working right now, and total input+output tokens spent today.

### Notes

- All figures are factual. Crabby never estimates progress percentages or
  completion times, and omits any field it cannot read reliably rather than
  showing a placeholder.
- Token counts are input + output (cache tokens excluded); per-workspace is the
  current session total, the global summary is today's total.
- The activity label is best-effort: it is derived from the tail of the
  transcript and can lag the live terminal by a moment, so it is only shown
  while the session is actively producing output. The coarse state
  (running/waiting/stopped) is always reliable.
- Transcripts are read incrementally (only newly-appended bytes) so the
  dashboard stays responsive with many workspaces.

## [0.5.1] - 2026-07-13

### Changed

- **`crabby import` no longer requires a git repository.** Any folder containing
  a `CLAUDE.md` is now importable; git is only used to show a branch for context
  when the folder happens to be a repository. Scanning stops at the first
  `CLAUDE.md` on each branch of the tree, so a project's own subdirectory context
  files aren't imported as separate workspaces.

### Fixed

- **Windows installer now upgrades an existing install.** `install.ps1` used to
  skip the Linux binary whenever a `crabby` was already present in WSL, so
  re-running the installer never updated it. It now always runs `install.sh`,
  which overwrites in place with the latest release.
- `install.sh` no longer prints `tmp: unbound variable` on exit; the cleanup
  trap referenced a function-local variable that was out of scope by the time it
  ran under `set -u`.

## [0.5.0] - 2026-07-13

### Added

- **`crabby import`.** Adopt existing Claude Code repositories in bulk. It scans
  a directory tree for git repositories that already contain a `CLAUDE.md` and
  presents them in an interactive selector (space to choose, Enter to import) —
  so a folder full of repos becomes Crabby workspaces in seconds. Already-imported
  projects are shown as such and can't be added twice. Scan the current directory
  with `crabby import`, or a specific one with `crabby import <path>`.
- **`crabby init <path>`.** `init` now takes an optional directory, so a project
  can be initialized without `cd`-ing into it first. With no argument it still
  initializes the current directory.

### Changed

- Imported workspaces behave identically to ones created with `crabby init`; the
  dashboard makes no distinction.

## [0.4.3] - 2026-07-12

### Changed

- **New visual identity.** Redesigned crab ASCII, wordmark `C R A B B Y`,
  tagline "Many Claudes. One shell.", and support line
  "prepare projects · orchestrate claude · one terminal", with a dedicated
  colour palette. Rendered from a single shared component across the dashboard,
  the pack selector, `crabby --help`, and the README.
- The dashboard and selector now centre horizontally on the real terminal width
  (from Bubble Tea's window-size events).

## [0.4.2] - 2026-07-11

### Changed

- `crabby --help` now opens with the same shared header (crab logo, name,
  slogan, author) shown on the dashboard and pack selector, so the help output
  matches the rest of the application.

## [0.4.1] - 2026-07-11

### Changed

- **Shared application header.** The crab-army logo, name, slogan
  ("Prepare projects. Organize sessions. Just one terminal."), and author
  attribution are now one reusable component rendered on every full-screen
  interface — the dashboard and the pack selector — so Crabby feels cohesive.
  The ASCII art is defined exactly once.
- **Consistent identity.** `crabby version`, `crabby doctor`, and `--help` now
  show the name, version, author (`marioolf`), and repo
  (`github.com/marioolf/crabby`) tastefully. Installers print the same line.

No functional changes.

## [0.4.0] - 2026-07-11

### Added

- **Dashboard.** The home screen is now a live dashboard with a compact,
  colourful "crab army" ASCII logo, projects sorted by importance
  (Running → Waiting → Stopped, then alphabetically), and colour-coded states
  (green / yellow / grey) with symbols.
- **Auto-refresh** (~1s) so state stays current without pressing `r`. State for
  all sessions is fetched in a single tmux call, so it stays fast with many
  projects.
- **Working indicator + bell.** A background session that is producing output is
  tagged `· working`; when one starts working, Crabby rings a short terminal
  bell — a lightweight, reliable notification that needs no background daemon.
- **Friendly empty state** when no projects are registered.
- Two more example packs (`examples/go-pack`, `examples/docs-pack`) and the
  packs directory is now surfaced in the pack selector and after `crabby init`,
  so packs are discoverable.

### Changed

- Project rows are cleaner: name, git branch, and state only (`less is more`).
- README documents the dashboard, visual identity, notifications (and their one
  honest limitation), and how to discover and create packs.

### Notes

- Desktop notifications *while attached to a session* are intentionally not
  implemented: Crabby yields the terminal to the session and does not run a
  background service. The live dashboard is the practical alternative.

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

[Unreleased]: https://github.com/marioolf/crabby/compare/v0.4.3...HEAD
[0.4.3]: https://github.com/marioolf/crabby/compare/v0.4.2...v0.4.3
[0.4.2]: https://github.com/marioolf/crabby/compare/v0.4.1...v0.4.2
[0.4.1]: https://github.com/marioolf/crabby/compare/v0.4.0...v0.4.1
[0.4.0]: https://github.com/marioolf/crabby/compare/v0.3.1...v0.4.0
[0.3.1]: https://github.com/marioolf/crabby/compare/v0.3.0...v0.3.1
[0.3.0]: https://github.com/marioolf/crabby/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/marioolf/crabby/compare/v0.1.1...v0.2.0
[0.1.1]: https://github.com/marioolf/crabby/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/marioolf/crabby/releases/tag/v0.1.0
