#!/usr/bin/env bash
#
# Crabby installer for Linux / WSL.
#
#   curl -fsSL https://raw.githubusercontent.com/marioolf/crabby/main/install.sh | bash
#
# Environment overrides:
#   CRABBY_REPO         owner/repo to download from   (default: marioolf/crabby)
#   CRABBY_INSTALL_DIR  where to install the binary    (default: ~/.local/bin)

set -euo pipefail

REPO="${CRABBY_REPO:-marioolf/crabby}"
INSTALL_DIR="${CRABBY_INSTALL_DIR:-$HOME/.local/bin}"
BINARY="crabby"

info() { printf '   %s\n' "$*"; }
ok()   { printf '\033[32m✓\033[0m %s\n' "$*"; }
warn() { printf '\033[33m!\033[0m %s\n' "$*"; }
err()  { printf '\033[31m✗\033[0m %s\n' "$*" >&2; }

main() {
	echo "🦀 Installing Crabby..."

	case "$(uname -m)" in
		x86_64 | amd64) arch=amd64 ;;
		aarch64 | arm64) arch=arm64 ;;
		*) err "Unsupported architecture: $(uname -m)"; exit 1 ;;
	esac

	local asset="${BINARY}_linux_${arch}.tar.gz"
	local url="https://github.com/${REPO}/releases/latest/download/${asset}"

	local tmp
	tmp="$(mktemp -d)"
	trap 'rm -rf "$tmp"' EXIT

	info "Downloading ${asset}..."
	if ! curl -fsSL "$url" -o "$tmp/$asset"; then
		err "Download failed: $url"
		err "Check that a release exists at https://github.com/${REPO}/releases"
		exit 1
	fi

	tar -xzf "$tmp/$asset" -C "$tmp"
	mkdir -p "$INSTALL_DIR" "$HOME/.local/share/crabby" "$HOME/.config/crabby"
	install -m 0755 "$tmp/$BINARY" "$INSTALL_DIR/$BINARY"
	ok "Installed crabby to $INSTALL_DIR/$BINARY"

	# PATH check
	case ":$PATH:" in
		*":$INSTALL_DIR:"*) ok "$INSTALL_DIR is on your PATH" ;;
		*)
			warn "$INSTALL_DIR is not on your PATH"
			info "Add this line to your shell profile (~/.bashrc or ~/.profile):"
			info "  export PATH=\"$INSTALL_DIR:\$PATH\""
			;;
	esac

	# Dependencies
	if command -v tmux >/dev/null 2>&1; then
		ok "tmux found"
	else
		warn "tmux not found — install it with: sudo apt install tmux"
	fi

	if command -v claude >/dev/null 2>&1; then
		ok "Claude Code found"
	else
		warn "Claude Code not found — see https://claude.com/claude-code"
	fi

	echo
	ok "Done! Run 'crabby doctor' to verify your setup."
	info "Crabby · by marioolf · github.com/marioolf/crabby"
}

main "$@"
