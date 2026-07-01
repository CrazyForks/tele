#!/usr/bin/env bash
# Local release driver: validate, update CHANGELOG, commit, and tag.
# Usage: scripts/release.sh <version|patch|minor|major>
# Does NOT push; prints the push command to run after review.
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"
# shellcheck source=scripts/changelog-lib.sh
source scripts/changelog-lib.sh
# shellcheck source=scripts/release-lib.sh
source scripts/release-lib.sh

CHANGELOG="CHANGELOG.md"
die() { echo "error: $*" >&2; exit 1; }

{ [ $# -ge 1 ] && [ $# -le 2 ]; } \
  || die "usage: scripts/release.sh <version|patch|minor|major> | beta [patch|minor|major]"

# Preconditions.
[ -z "$(git status --porcelain)" ] || die "working tree is not clean"
[ "$(git rev-parse --abbrev-ref HEAD)" = "main" ] || die "not on main branch"

# Latest stable tag only — beta prereleases (which contain "-") never bump the base.
latest=$(git tag -l 'v*' --sort=-version:refname | grep -Ev '\-' | head -n1)
[ -n "$latest" ] || latest="v0.0.0"

# --- beta channel: cut vX.Y.Z-beta.N without touching CHANGELOG ---
if [ "$1" = "beta" ]; then
  bump=${2:-patch}
  base=$(compute_target_version "$latest" "$bump")
  tag=$(next_beta_tag "$base" "$(git tag -l 'v*')")

  [ "$(release_tag_kind "$tag")" = "beta" ] \
    || die "computed beta tag is malformed: $tag"

  changelog_unreleased_has_content "$CHANGELOG" \
    || die "nothing to release: [Unreleased] in $CHANGELOG is empty"

  confirm_release "$latest" "$tag" "beta channel" || die "aborted"

  # Beta is a preview of the upcoming stable; leave [Unreleased] intact and do
  # not commit. Release notes come from GoReleaser's github changelog.
  git tag -a "$tag" -m "$tag"
  echo "Tagged $tag (beta prerelease)."
  echo "Review the tag, then run: git push origin main --follow-tags"
  exit 0
fi

# --- stable channel ---
version=$(compute_target_version "$latest" "$1")
tag="v$version"

[ "$(release_tag_kind "$tag")" = "stable" ] \
  || die "stable release tag must be vX.Y.Z (got $tag); use 'beta' for prereleases"

git rev-parse "$tag" >/dev/null 2>&1 && die "tag $tag already exists"
changelog_unreleased_has_content "$CHANGELOG" \
  || die "nothing to release: [Unreleased] in $CHANGELOG is empty"

confirm_release "$latest" "$tag" "stable" || die "aborted"

changelog_promote "$CHANGELOG" "$version" "$(date +%F)"
body=$(changelog_extract_body "$CHANGELOG" "$version")

git add "$CHANGELOG"
git commit -m "chore: release $tag"
printf '%s\n\n%s\n' "$tag" "$body" | git tag -a "$tag" -F -

echo "Tagged $tag."
echo "Review the commit and tag, then run: git push origin main --follow-tags"
