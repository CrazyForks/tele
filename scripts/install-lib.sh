#!/usr/bin/env bash
# Pure, sourceable helpers for scripts/install.sh. No side effects on source.

REPO="sorokin-vladimir/tele"

# normalize_os maps a `uname -s` value to a goreleaser OS token.
normalize_os() { # normalize_os <uname-s>
  case "$1" in
    Linux)   echo linux ;;
    Darwin)  echo darwin ;;
    FreeBSD) echo freebsd ;;
    OpenBSD) echo openbsd ;;
    NetBSD)  echo netbsd ;;
    *) echo "unsupported OS: $1" >&2; return 1 ;;
  esac
}

# normalize_arch maps a `uname -m` value to a goreleaser arch token.
normalize_arch() { # normalize_arch <uname-m>
  case "$1" in
    x86_64|amd64)   echo amd64 ;;
    aarch64|arm64)  echo arm64 ;;
    *) echo "unsupported architecture: $1" >&2; return 1 ;;
  esac
}

# detect_os / detect_arch resolve the running host.
detect_os()   { normalize_os "$(uname -s)"; }
detect_arch() { normalize_arch "$(uname -m)"; }

# asset_name builds the raw-binary asset name published by goreleaser.
asset_name() { # asset_name <os> <arch>
  echo "tele-$1-$2"
}

# asset_url builds the GitHub Releases download URL. version is a tag (vX.Y.Z)
# or the literal "latest".
asset_url() { # asset_url <version> <os> <arch>
  local version="$1" os="$2" arch="$3" name
  name="$(asset_name "$os" "$arch")"
  if [ "$version" = "latest" ]; then
    echo "https://github.com/${REPO}/releases/latest/download/${name}"
  else
    echo "https://github.com/${REPO}/releases/download/${version}/${name}"
  fi
}

# latest_prerelease_tag extracts the newest prerelease tag from the GitHub
# releases API JSON (passed as an argument or on stdin). The API returns
# releases newest-first and emits "tag_name" before "prerelease" within each
# object, so we remember the last tag seen and print it at the first
# prerelease:true. Prints nothing when there is no prerelease. Avoids a jq/gh
# dependency so the installer stays a self-contained curl | sh.
latest_prerelease_tag() { # latest_prerelease_tag [json]
  local json="${1:-}"
  if [ -z "$json" ]; then json="$(cat)"; fi
  printf '%s\n' "$json" | awk '
    /"tag_name":/ {
      tag = $0
      sub(/.*"tag_name":[[:space:]]*"/, "", tag)
      sub(/".*/, "", tag)
    }
    /"prerelease":[[:space:]]*true/ { print tag; exit }
  '
}
