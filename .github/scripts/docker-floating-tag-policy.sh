#!/usr/bin/env bash
set -euo pipefail

IMAGE_REPO="${IMAGE_REPO:-gtstef/filebrowser}"
SLIM_SUFFIX="${SLIM_SUFFIX:-}"

if [[ -z "${GITHUB_OUTPUT:-}" ]]; then
  echo "GITHUB_OUTPUT is not set" >&2
  exit 1
fi

write_outputs() {
  local update_track="$1"
  local update_major="$2"
  echo "update_track=${update_track}" >> "$GITHUB_OUTPUT"
  echo "update_major=${update_major}" >> "$GITHUB_OUTPUT"
}

registry_version() {
  local tag="$1"
  local ver err err_file
  err_file="$(mktemp)"
  if ! ver="$(docker buildx imagetools inspect "${IMAGE_REPO}:${tag}" \
    --format '{{ with index .Image "linux/amd64" }}{{ index .Config.Labels "org.opencontainers.image.version" }}{{ end }}' 2>"$err_file")"; then
    err="$(cat "$err_file")"
    rm -f "$err_file"
    if grep -qiE 'not found|manifest unknown' <<<"$err"; then
      ver=""
    else
      echo "Failed to inspect ${IMAGE_REPO}:${tag}: ${err}" >&2
      exit 1
    fi
  else
    rm -f "$err_file"
    if [[ -z "$ver" ]]; then
      echo "Missing org.opencontainers.image.version on ${IMAGE_REPO}:${tag} (linux/amd64)" >&2
      exit 1
    fi
  fi
  ver="${ver#v}"
  printf '%s' "$ver"
}

semver_allows_update() {
  local new="$1"
  local incumbent="$2"
  if [[ -z "$incumbent" ]]; then
    return 0
  fi
  local winner
  winner="$(printf '%s\n' "$incumbent" "$new" | sort -V | tail -n1)"
  [[ "$winner" == "$new" ]]
}

ref="${GITHUB_REF_NAME:-}"
ref="${ref#v}"

if [[ ! "$ref" =~ ^([0-9]+\.[0-9]+\.[0-9]+)-(.+)$ ]]; then
  echo "Tag '${GITHUB_REF_NAME:-}' has no X.Y.Z-suffix form; leaving major-only floats enabled, track floats disabled"
  write_outputs false true
  exit 0
fi

new_version="$ref"
semver_core="${BASH_REMATCH[1]}"
suffix="${BASH_REMATCH[2]}"
major="$(echo "$semver_core" | cut -d. -f1)"

track=""
case "$suffix" in
  beta) track="beta" ;;
  stable) track="stable" ;;
  *)
    echo "Tag suffix '${suffix}' is not beta/stable; leaving major-only floats enabled, track floats disabled"
    write_outputs false true
    exit 0
    ;;
esac

track_tag="${track}${SLIM_SUFFIX}"
major_tag="${major}-${track}${SLIM_SUFFIX}"

incumbent_track="$(registry_version "$track_tag")"
incumbent_major="$(registry_version "$major_tag")"

update_track=false
if semver_allows_update "$new_version" "$incumbent_track"; then
  update_track=true
fi

update_major=false
if semver_allows_update "$new_version" "$incumbent_major"; then
  update_major=true
fi

echo "Floating tag policy (${SLIM_SUFFIX:-full} image):"
echo "  New version: ${new_version}"
echo "  Track tag: ${track_tag} (incumbent: ${incumbent_track:-<none>}) -> update_track=${update_track}"
echo "  Major tag: ${major_tag} (incumbent: ${incumbent_major:-<none>}) -> update_major=${update_major}"

write_outputs "$update_track" "$update_major"
