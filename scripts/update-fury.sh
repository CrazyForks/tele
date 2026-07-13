#!/usr/bin/env bash
#
# Upload the deb/rpm/apk packages built by `goreleaser release` to Gemfury, so
# users can `apt install` / `dnf install` / `apk add` tele from a hosted repo.
#
# GoReleaser's `furies` publisher is Pro-only, so this script mirrors the
# scripts/update-formula.sh pattern instead. Reads dist/*.deb, dist/*.rpm and
# dist/*.apk, so it MUST run after goreleaser in the same job.
#
# Only stable (vX.Y.Z) tags publish; beta tags are skipped so a prerelease never
# replaces the stable package in the repo.
#
# Usage: scripts/update-fury.sh <tag>
# Requires: FURY_TOKEN in the environment (Gemfury push token).
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"
# shellcheck source=scripts/release-lib.sh
source scripts/release-lib.sh

tag=${1:?usage: scripts/update-fury.sh <tag>}
: "${FURY_TOKEN:?FURY_TOKEN is not set}"

if [[ "$(release_tag_kind "$tag")" != stable ]]; then
  echo "update-fury: ${tag} is not a stable tag, skipping Gemfury upload"
  exit 0
fi

shopt -s nullglob
packages=(dist/*.deb dist/*.rpm dist/*.apk)
if (( ${#packages[@]} == 0 )); then
  echo "update-fury: no .deb/.rpm/.apk found in dist/" >&2
  exit 1
fi

for pkg in "${packages[@]}"; do
  echo "update-fury: uploading ${pkg}"
  curl -sf -F package=@"${pkg}" "https://push.fury.io/${FURY_TOKEN}/" >/dev/null
done

echo "update-fury: uploaded ${#packages[@]} package(s) for ${tag}"
