#!/usr/bin/env bash
# Emit GoReleaser release header + name for the given tag.
# Appends RELEASE_NAME / RELEASE_HEADER to $GITHUB_ENV, which GoReleaser reads
# via .Env in .goreleaser.yaml. Usage: scripts/changelog-notes.sh <tag>
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"
# shellcheck source=scripts/changelog-lib.sh
source scripts/changelog-lib.sh
# shellcheck source=scripts/release-lib.sh
source scripts/release-lib.sh

tag=${1:?usage: scripts/changelog-notes.sh <tag>}
version=${tag#v}

# Earliest guard in the Release workflow: reject anything that is neither a
# stable (vX.Y.Z) nor a beta (vX.Y.Z-beta.N) tag before goreleaser runs.
kind=$(release_tag_kind "$tag")
[ "$kind" != "invalid" ] \
  || { echo "error: unexpected release tag $tag (want vX.Y.Z or vX.Y.Z-beta.N)" >&2; exit 1; }

body=$(changelog_extract_body "CHANGELOG.md" "$version")
title=$(changelog_extract_title "CHANGELOG.md" "$version")

if [ -n "$title" ]; then
  name="[$version] $title"
elif [ "$kind" = "beta" ]; then
  # Beta prerelease: no CHANGELOG entry; label the channel clearly.
  name="$tag (beta)"
else
  name="$tag"
fi

{
  echo "RELEASE_NAME=$name"
  echo "RELEASE_HEADER<<__CHANGELOG_EOF__"
  printf '%s\n' "$body"
  echo "__CHANGELOG_EOF__"
} >> "${GITHUB_ENV:?GITHUB_ENV is not set}"
