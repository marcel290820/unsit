#!/bin/sh
# Runs install.sh against fake releases: a good one installs, a tampered or
# broken one is refused and leaves the installed unsit alone.

set -eu

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
sed "s|^	url=.*|	url=\"file://$work/release\"|" "$(dirname "$0")/install.sh" >"$work/install.sh"
export UNSIT_INSTALL_DIR="$work/bin"

sum() { if command -v sha256sum >/dev/null 2>&1; then sha256sum "$@"; else shasum -a 256 "$@"; fi; }

# release publishes an unsit that runs $1, for every platform install.sh knows.
release() {
	rm -rf "$work/release" && mkdir -p "$work/release"
	printf '#!/bin/sh\n%s\n' "$1" >"$work/unsit" && chmod 755 "$work/unsit"
	for target in linux_amd64 linux_arm64 darwin_amd64 darwin_arm64; do
		tar -czf "$work/release/unsit_$target.tar.gz" -C "$work" unsit
	done
	(cd "$work/release" && sum ./*.tar.gz | sed 's| \./| |' >checksums.txt)
}

expect() { # expect <case> installs|refuses <version installed afterwards>
	if ! out="$(sh "$work/install.sh" 2>&1)" && [ "$2" = installs ]; then
		echo "FAIL $1: $out" && exit 1
	elif [ "$2" = refuses ] && [ "$(echo "$out" | grep -c 'not installing')" != 1 ]; then
		echo "FAIL $1 was not refused: $out" && exit 1
	elif [ "$("$UNSIT_INSTALL_DIR/unsit" --version)" != "$3" ]; then
		echo "FAIL $1: installed $("$UNSIT_INSTALL_DIR/unsit" --version), want $3" && exit 1
	fi
	echo "ok   $1"
}

release 'echo unsit 1.0.0'
expect "a good release" installs "unsit 1.0.0"

release 'echo unsit 2.0.0'
for archive in "$work"/release/*.tar.gz; do echo tampered >>"$archive"; done
expect "a tampered release" refuses "unsit 1.0.0"

release 'exit 1'
expect "a binary that does not run" refuses "unsit 1.0.0"
