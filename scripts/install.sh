#!/usr/bin/env bash
#
# Install tele by auto-detecting OS/arch and downloading the matching binary
# from GitHub Releases. Usage:
#   curl -sL https://raw.githubusercontent.com/sorokin-vladimir/tele/main/scripts/install.sh | sh
#   curl -sL .../install.sh | sh -s -- --beta        # latest prerelease
# Options:
#   --beta            install the latest beta (prerelease) as `tele-beta`
#   --version <tag>   install a specific tag, e.g. v1.9.0 (overrides --beta)
# Env (equivalent to the flags):
#   TELE_CHANNEL   stable | beta   (default: stable)
#   TELE_VERSION   tag to install  (overrides the channel)
#   PREFIX         install dir (default: /usr/local/bin, else ~/.local/bin)
set -euo pipefail

# Resolve the library whether run from a checkout or piped from curl.
if [ -f "$(dirname "$0")/install-lib.sh" ]; then
  # shellcheck source=scripts/install-lib.sh
  . "$(dirname "$0")/install-lib.sh"
else
  eval "$(curl -fsSL "https://raw.githubusercontent.com/sorokin-vladimir/tele/main/scripts/install-lib.sh")"
fi

channel="${TELE_CHANNEL:-stable}"
version="${TELE_VERSION:-}"

while [ $# -gt 0 ]; do
  case "$1" in
    --beta) channel=beta ;;
    --version) shift; version="${1:-}" ;;
    --version=*) version="${1#*=}" ;;
    *) echo "unknown option: $1" >&2; exit 1 ;;
  esac
  shift
done

os="$(detect_os)"
arch="$(detect_arch)"

# Resolve the tag to download and the installed binary name. An explicit
# --version always wins; otherwise the channel decides. Beta installs as a
# distinct `tele-beta` binary so it coexists with a stable `tele`.
binname=tele
if [ -n "$version" ]; then
  tag="$version"
elif [ "$channel" = "beta" ]; then
  binname=tele-beta
  json="$(curl -fsSL "https://api.github.com/repos/sorokin-vladimir/tele/releases?per_page=30")"
  tag="$(latest_prerelease_tag "$json")"
  [ -n "$tag" ] || { echo "no beta (prerelease) available" >&2; exit 1; }
else
  tag="latest"
fi

url="$(asset_url "$tag" "$os" "$arch")"

# Choose an install dir: system-wide if writable, else the per-user fallback.
dest="${PREFIX:-/usr/local/bin}"
if [ ! -d "$dest" ] || [ ! -w "$dest" ]; then
  dest="${HOME}/.local/bin"
  mkdir -p "$dest"
fi

tmp="$(mktemp)"
trap 'rm -f "$tmp"' EXIT
echo "Downloading $binname ($os/$arch, $tag)..." >&2
curl -fsSL "$url" -o "$tmp"
chmod +x "$tmp"
mv "$tmp" "$dest/$binname"
trap - EXIT

echo "Installed $binname to $dest/$binname" >&2
case ":$PATH:" in
  *":$dest:"*) ;;
  *) echo "Note: $dest is not on your PATH; add it to run '$binname' directly." >&2 ;;
esac
