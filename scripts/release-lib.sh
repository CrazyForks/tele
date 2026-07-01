#!/usr/bin/env bash
# Pure helpers for scripts/release.sh: version math and the confirmation gate.
# Sourced, not executed. No git side effects.

# compute_target_version <latest_stable> <bump>
# bump is major|minor|patch or an explicit X.Y.Z / vX.Y.Z. Echoes X.Y.Z.
compute_target_version() {
  local latest=$1 bump=$2 major minor patch
  IFS=. read -r major minor patch <<<"${latest#v}"
  case "$bump" in
    major) echo "$((major + 1)).0.0" ;;
    minor) echo "${major}.$((minor + 1)).0" ;;
    patch) echo "${major}.${minor}.$((patch + 1))" ;;
    *)     echo "${bump#v}" ;;
  esac
}

# next_beta_tag <base_version> <existing_tags_newline>
# base_version is X.Y.Z; existing_tags is a newline-separated tag list.
# Echoes vX.Y.Z-beta.N with N one past the highest existing beta for that base.
next_beta_tag() {
  local base=$1 tags=$2 t n max=0
  while IFS= read -r t; do
    case "$t" in
      "v${base}-beta."*)
        n=${t##*.}
        if [[ $n =~ ^[0-9]+$ ]] && (( n > max )); then
          max=$n
        fi
        ;;
    esac
  done <<<"$tags"
  echo "v${base}-beta.$((max + 1))"
}

# release_tag_kind <tag>
# Single source of truth for tag classification. Echoes:
#   stable   for vX.Y.Z
#   beta     for vX.Y.Z-beta.N
#   invalid  for anything else (other prereleases, malformed tags)
release_tag_kind() {
  local tag=$1
  if [[ $tag =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    echo stable
  elif [[ $tag =~ ^v[0-9]+\.[0-9]+\.[0-9]+-beta\.[0-9]+$ ]]; then
    echo beta
  else
    echo invalid
  fi
}

# confirm_release <current_tag> <new_tag> <channel>
# Prints a summary to stderr, reads one line from stdin. Returns 0 only on y/Y.
confirm_release() {
  local current=$1 newtag=$2 channel=$3 reply
  {
    echo "current version: ${current}"
    printf 'new tag:         %s   (%s)\n' "$newtag" "$channel"
    printf 'proceed? [y/N] '
  } >&2
  read -r reply
  [[ $reply == y || $reply == Y ]]
}
