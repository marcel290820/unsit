#!/bin/sh
# Installs the latest unsit release for macOS or Linux into ~/.local/bin
# (override with UNSIT_INSTALL_DIR). No sudo; writes nothing else.
#
#   curl -fsSL https://raw.githubusercontent.com/marcel290820/unsit/main/install.sh | sh
#
# Uninstall: rm ~/.local/bin/unsit

set -eu

repo="marcel290820/unsit"
dir="${UNSIT_INSTALL_DIR:-$HOME/.local/bin}"

fail() {
	echo "unsit install: $*" >&2
	exit 1
}

main() {
	case "$(uname -s)" in
	Linux) os=linux ;;
	Darwin) os=darwin ;;
	*) fail "unsupported system $(uname -s); on Windows use install.ps1" ;;
	esac
	case "$(uname -m)" in
	x86_64 | amd64) arch=amd64 ;;
	arm64 | aarch64) arch=arm64 ;;
	*) fail "unsupported CPU $(uname -m)" ;;
	esac

	if command -v sha256sum >/dev/null 2>&1; then
		sha256="sha256sum"
	elif command -v shasum >/dev/null 2>&1; then
		sha256="shasum -a 256"
	else
		fail "need sha256sum or shasum to verify the download"
	fi

	archive="unsit_${os}_${arch}.tar.gz"
	url="https://github.com/$repo/releases/latest/download"
	tmp="$(mktemp -d)"
	staged=""
	trap 'rm -rf "$tmp"; [ -z "$staged" ] || rm -f "$staged"' EXIT

	echo "Downloading $archive"
	curl -fsSL -o "$tmp/$archive" "$url/$archive" || fail "download failed: $url/$archive"
	curl -fsSL -o "$tmp/checksums.txt" "$url/checksums.txt" || fail "download failed: $url/checksums.txt"

	want="$(awk -v f="$archive" '$2 == f { print $1 }' "$tmp/checksums.txt")"
	got="$($sha256 "$tmp/$archive" | awk '{ print $1 }')"
	if [ -z "$want" ] || [ "$want" != "$got" ]; then
		fail "checksum mismatch for $archive; not installing"
	fi

	tar -xzf "$tmp/$archive" -C "$tmp" unsit
	mkdir -p "$dir"
	# Stage next to the target and rename over it: a running unsit keeps
	# its old file instead of having it rewritten underneath it.
	staged="$(mktemp "$dir/.unsit.XXXXXX")"
	cp "$tmp/unsit" "$staged"
	chmod 755 "$staged"
	version="$("$staged" --version)" || fail "the downloaded unsit does not run on this system; not installing"
	mv -f "$staged" "$dir/unsit"
	staged=""
	echo "Installed $version to $dir/unsit"

	case ":$PATH:" in
	*":$dir:"*) echo "Run: unsit" ;;
	*)
		echo
		echo "$dir is not on your PATH. Add this line to your shell's startup file"
		echo "(~/.zshrc on macOS, ~/.bashrc on most Linux), then open a new terminal:"
		echo
		echo "  export PATH=\"$dir:\$PATH\""
		;;
	esac
}

# Everything runs from main, so a download cut short cannot run half a script.
main
