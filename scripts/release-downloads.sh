#!/usr/bin/env bash
#
# Show download counts for all releases of a given major version.
# Prints per-tag totals (summed across all platform assets) and a grand total.
#
# Usage:
#   ./release-downloads.sh 0        # all v0.x.y releases
#   ./release-downloads.sh v1       # all v1.x.y releases
#
set -euo pipefail

if [[ $# -lt 1 ]]; then
  echo "usage: $0 <major-version>   e.g. 0  or  v1" >&2
  exit 1
fi

# Normalize: accept both "0" and "v0", reduce to the bare number.
major="${1#v}"

# Pull every release tag, keep only those matching the requested major version.
tags="$(gh release list --limit 200 --json tagName --jq '.[].tagName' \
  | grep -E "^v?${major}\." || true)"

if [[ -z "$tags" ]]; then
  echo "no releases found for major version v${major}" >&2
  exit 1
fi

grand_total=0

while IFS= read -r tag; do
  [[ -z "$tag" ]] && continue
  # Sum downloadCount across all assets of this tag; empty asset list -> 0.
  count="$(gh release view "$tag" --json assets \
    --jq '[.assets[].downloadCount] | add // 0')"
  printf '%-12s %d\n' "$tag" "$count"
  grand_total=$(( grand_total + count ))
done <<< "$tags"

printf '%-12s %d\n' "TOTAL" "$grand_total"
