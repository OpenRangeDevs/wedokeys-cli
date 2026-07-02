#!/bin/sh
# wdk installer — downloads the latest release binary from GitHub and installs it.
#
#   curl -fsSL https://raw.githubusercontent.com/OpenRangeDevs/wedokeys-cli/main/install.sh | sh
#
# Override the install directory with WDK_INSTALL_DIR (default ~/.local/bin).
set -eu

REPO="OpenRangeDevs/wedokeys-cli"
BINDIR="${WDK_INSTALL_DIR:-$HOME/.local/bin}"

# --- detect platform ---------------------------------------------------------
os="$(uname -s)"
arch="$(uname -m)"
case "$os" in
	Darwin) os="darwin" ;;
	Linux) os="linux" ;;
	*) echo "wdk: unsupported OS: $os" >&2; exit 1 ;;
esac
case "$arch" in
	x86_64 | amd64) arch="amd64" ;;
	arm64 | aarch64) arch="arm64" ;;
	*) echo "wdk: unsupported architecture: $arch" >&2; exit 1 ;;
esac

# --- resolve the latest release tag -----------------------------------------
tag="$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" \
	| grep '"tag_name"' | head -1 | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')"
if [ -z "$tag" ]; then
	echo "wdk: could not determine the latest release" >&2
	exit 1
fi
version="${tag#v}"
archive="wdk_${version}_${os}_${arch}.tar.gz"
base="https://github.com/$REPO/releases/download/$tag"

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

echo "wdk: downloading $archive ($tag)…"
curl -fsSL "$base/$archive" -o "$tmp/$archive"
curl -fsSL "$base/checksums.txt" -o "$tmp/checksums.txt"

# --- verify checksum ---------------------------------------------------------
expected="$(grep " ${archive}\$" "$tmp/checksums.txt" | awk '{print $1}')"
if [ -z "$expected" ]; then
	echo "wdk: no checksum found for $archive" >&2
	exit 1
fi
if command -v sha256sum >/dev/null 2>&1; then
	actual="$(sha256sum "$tmp/$archive" | awk '{print $1}')"
elif command -v shasum >/dev/null 2>&1; then
	actual="$(shasum -a 256 "$tmp/$archive" | awk '{print $1}')"
else
	echo "wdk: no sha256 tool (sha256sum/shasum) found" >&2
	exit 1
fi
if [ "$expected" != "$actual" ]; then
	echo "wdk: checksum mismatch for $archive" >&2
	exit 1
fi

# --- install -----------------------------------------------------------------
tar -xzf "$tmp/$archive" -C "$tmp"
mkdir -p "$BINDIR"
install -m 0755 "$tmp/wdk" "$BINDIR/wdk"
install -m 0755 "$tmp/kamal-secrets-wedokeys" "$BINDIR/kamal-secrets-wedokeys"

echo "wdk: installed wdk $version to $BINDIR/wdk"

case ":$PATH:" in
	*":$BINDIR:"*) ;;
	*) echo "wdk: note — $BINDIR is not on your PATH. Add it with:"
	   echo "       export PATH=\"$BINDIR:\$PATH\"" ;;
esac

# --- shell completions ---------------------------------------------------------
# Install tab completion where the user's shell auto-loads it by convention.
# Never edits rc files; falls back to printing the manual one-liner.
install_completions() {
	shell_name="$(basename "${SHELL:-}")"
	case "$shell_name" in
	zsh)
		if [ -d "$HOME/.oh-my-zsh" ]; then
			mkdir -p "$HOME/.oh-my-zsh/completions"
			"$BINDIR/wdk" completion zsh > "$HOME/.oh-my-zsh/completions/_wdk"
			echo "wdk: tab completion installed (open a new terminal to activate)"
		else
			echo "wdk: enable tab completion with:"
			echo "       echo 'source <(wdk completion zsh)' >> ~/.zshrc"
		fi
		;;
	bash)
		comp_dir="${XDG_DATA_HOME:-$HOME/.local/share}/bash-completion/completions"
		mkdir -p "$comp_dir"
		"$BINDIR/wdk" completion bash > "$comp_dir/wdk"
		echo "wdk: tab completion installed (open a new terminal to activate)"
		;;
	fish)
		mkdir -p "$HOME/.config/fish/completions"
		"$BINDIR/wdk" completion fish > "$HOME/.config/fish/completions/wdk.fish"
		echo "wdk: tab completion installed (open a new terminal to activate)"
		;;
	*)
		echo "wdk: enable tab completion with: wdk completion --help"
		;;
	esac
}
install_completions || echo "wdk: completion setup skipped (run 'wdk completion --help' to set it up manually)"
