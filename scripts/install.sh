#!/bin/sh
# Install gomddoc from GitHub Releases.
#
#   curl -fsSL https://raw.githubusercontent.com/monolithiclab/gomddoc/main/scripts/install.sh | sh
#
# Environment:
#   GOMDDOC_VERSION      version to install, with or without the leading "v" (default: latest)
#   GOMDDOC_INSTALL_DIR  target directory (default: /usr/local/bin, falling back to ~/.local/bin)
#
# The downloaded archive is verified against the release's SHA256SUMS before install.
# Checksums are also cosign-signed; see the README for signature verification.

set -eu

REPO="monolithiclab/gomddoc"
BINARY="gomddoc"

die() {
	printf '\033[1;31merror\033[0m: %s\n' "$1" >&2
	exit 1
}

info() {
	printf '\033[1;34m==>\033[0m %s\n' "$1"
}

warn() {
	printf '\033[1;33mwarning\033[0m: %s\n' "$1" >&2
}

need() {
	command -v "$1" >/dev/null 2>&1
}

# fetch <url> <dest>
fetch() {
	if need curl; then
		curl -fsSL "$1" -o "$2"
	elif need wget; then
		wget -qO "$2" "$1"
	else
		die "neither curl nor wget is available"
	fi
}

# fetch_stdout <url>
fetch_stdout() {
	if need curl; then
		curl -fsSL "$1"
	elif need wget; then
		wget -qO- "$1"
	else
		die "neither curl nor wget is available"
	fi
}

detect_os() {
	os=$(uname -s)
	case "$os" in
	Linux) echo linux ;;
	Darwin) echo darwin ;;
	*) die "unsupported operating system: $os (prebuilt binaries exist for Linux and macOS only)" ;;
	esac
}

detect_arch() {
	arch=$(uname -m)
	case "$arch" in
	x86_64 | amd64) echo amd64 ;;
	aarch64 | arm64) echo arm64 ;;
	*) die "unsupported architecture: $arch (prebuilt binaries exist for amd64 and arm64 only)" ;;
	esac
}

# Resolve the newest release tag via the GitHub API.
latest_version() {
	body=$(fetch_stdout "https://api.github.com/repos/${REPO}/releases/latest") ||
		die "could not reach the GitHub API to resolve the latest release"

	# Extract "tag_name": "vX.Y.Z" without requiring jq.
	tag=$(printf '%s' "$body" | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n 1)
	[ -n "$tag" ] || die "could not parse the latest release tag from the GitHub API response"
	printf '%s' "$tag"
}

# verify_checksum <archive_path> <archive_name> <sums_path>
verify_checksum() {
	expected=$(awk -v name="$2" '$2 == name || $2 == "*" name { print $1 }' "$3" | head -n 1)
	[ -n "$expected" ] || die "no checksum for $2 in SHA256SUMS"

	if need sha256sum; then
		actual=$(sha256sum "$1" | awk '{print $1}')
	elif need shasum; then
		actual=$(shasum -a 256 "$1" | awk '{print $1}')
	else
		warn "no sha256sum or shasum found; skipping checksum verification"
		return 0
	fi

	[ "$actual" = "$expected" ] ||
		die "checksum mismatch for $2 (expected $expected, got $actual)"
	info "Checksum verified."
}

# Pick an install directory and echo it, along with whether sudo is needed.
# Sets the globals INSTALL_DIR and NEED_SUDO.
resolve_install_dir() {
	if [ -n "${GOMDDOC_INSTALL_DIR:-}" ]; then
		INSTALL_DIR="$GOMDDOC_INSTALL_DIR"
	elif [ -w /usr/local/bin ] 2>/dev/null; then
		INSTALL_DIR=/usr/local/bin
	elif need sudo && [ -d /usr/local/bin ]; then
		INSTALL_DIR=/usr/local/bin
	else
		INSTALL_DIR="$HOME/.local/bin"
	fi

	NEED_SUDO=no
	mkdir -p "$INSTALL_DIR" 2>/dev/null || true
	[ -d "$INSTALL_DIR" ] || die "install directory does not exist and could not be created: $INSTALL_DIR"

	if [ ! -w "$INSTALL_DIR" ]; then
		if need sudo; then
			NEED_SUDO=yes
		else
			die "$INSTALL_DIR is not writable and sudo is unavailable; set GOMDDOC_INSTALL_DIR to a writable path"
		fi
	fi
}

warn_if_not_on_path() {
	case ":${PATH}:" in
	*":$1:"*) ;;
	*) warn "$1 is not on your PATH; add it with:  export PATH=\"$1:\$PATH\"" ;;
	esac
}

main() {
	os=$(detect_os)
	arch=$(detect_arch)

	if [ -n "${GOMDDOC_VERSION:-}" ]; then
		tag="$GOMDDOC_VERSION"
		# Accept both "0.1.1" and "v0.1.1".
		case "$tag" in
		v*) ;;
		*) tag="v$tag" ;;
		esac
	else
		info "Resolving latest release..."
		tag=$(latest_version)
	fi

	# Release archives use the version without the leading "v".
	version=${tag#v}
	archive="${BINARY}_${version}_${os}_${arch}.tar.gz"
	base="https://github.com/${REPO}/releases/download/${tag}"

	tmp=$(mktemp -d 2>/dev/null || mktemp -d -t gomddoc)
	trap 'rm -rf "$tmp"' EXIT INT TERM

	info "Downloading ${BINARY} ${tag} (${os}/${arch})..."
	fetch "${base}/${archive}" "${tmp}/${archive}" ||
		die "download failed: ${base}/${archive}"
	fetch "${base}/SHA256SUMS" "${tmp}/SHA256SUMS" ||
		die "could not download SHA256SUMS from ${base}"

	verify_checksum "${tmp}/${archive}" "$archive" "${tmp}/SHA256SUMS"

	tar -xzf "${tmp}/${archive}" -C "$tmp" ||
		die "could not extract $archive"
	[ -f "${tmp}/${BINARY}" ] || die "$BINARY not found in $archive"
	chmod +x "${tmp}/${BINARY}"

	resolve_install_dir
	info "Installing to ${INSTALL_DIR}/${BINARY}..."
	if [ "$NEED_SUDO" = yes ]; then
		warn "${INSTALL_DIR} needs elevated permissions; you may be prompted for your password."
		sudo install -m 0755 "${tmp}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
	else
		install -m 0755 "${tmp}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
	fi

	warn_if_not_on_path "$INSTALL_DIR"
	info "Installed $("${INSTALL_DIR}/${BINARY}" --version 2>/dev/null || echo "$BINARY $tag")"
	info "Get started:  ${BINARY} serve ./docs"
}

main "$@"
