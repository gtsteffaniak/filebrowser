#!/usr/bin/env bash
# Builds the comma-separated Docker tag list for a release image.
#
# Usage: docker-release-tags.sh <metadata.json> <variant> <update_track> <update_major>
#   metadata.json  JSON output of docker/metadata-action (uses .tags)
#   variant        default | slim | media
#                  default and media: floating tags include latest
#                  slim: every tag gets a -slim suffix and latest is not added
#   update_track   true when the semver policy allows floating beta/stable/latest tags
#   update_major   true when the semver policy allows major-only version tags
#
# Image repositories are fixed per variant so that each image only gets its own tags.
set -euo pipefail

if [[ $# -ne 4 ]]; then
  echo "usage: $0 <metadata.json> <default|slim|media> <true|false> <true|false>" >&2
  exit 2
fi

metadata_file=$1
variant=$2
update_track=$3
update_major=$4

case "$variant" in
default)
  suffix=""
  add_latest=true
  repos=(gtstef/filebrowser ghcr.io/gtsteffaniak/filebrowser)
  ;;
slim)
  suffix="-slim"
  add_latest=false
  repos=(gtstef/filebrowser ghcr.io/gtsteffaniak/filebrowser)
  ;;
media)
  suffix=""
  add_latest=true
  repos=(gtstef/filebrowser-media ghcr.io/gtsteffaniak/filebrowser-media)
  ;;
*)
  echo "unknown variant: $variant" >&2
  exit 2
  ;;
esac

jq -r \
  --arg suffix "$suffix" \
  --argjson addLatest "$add_latest" \
  --argjson updateTrack "$update_track" \
  --argjson updateMajor "$update_major" \
  --arg dh "${repos[0]}" \
  --arg gh "${repos[1]}" '
  # Strip the v prefix, drop metadata latest, and append the variant suffix.
  .tags |= map(
    sub(":v"; ":")
    | select(test(":latest") | not)
    | . + $suffix
  ) |
  # Version tags with a prerelease suffix (X.Y.Z-suffix) get major.minor, and major when allowed.
  .tags += [
    .tags[]
    | select(test(":[0-9]+\\.[0-9]+\\.[0-9]+-"))
    | split(":")
    | .[-1] as $version
    | ($version | split("-")) as $version_split
    | ($version_split[0] | split(".")) as $version_parts
    | ($version_split[1:] | join("-")) as $tag_suffix
    | ($version_parts | "\(.[0]).\(.[1])") as $major_minor
    | ($version_parts[0]) as $major
    | "\($dh):\($major_minor)-\($tag_suffix)",
      "\($gh):\($major_minor)-\($tag_suffix)",
      if $updateMajor then
        "\($dh):\($major)-\($tag_suffix)",
        "\($gh):\($major)-\($tag_suffix)"
      else empty end
  ] |
  # Floating beta tag.
  if $updateTrack and any(.tags[]; split(":")[-1] | endswith("-beta" + $suffix)) then
    .tags += ["\($dh):beta\($suffix)", "\($gh):beta\($suffix)"]
  else . end |
  # Floating stable tag, plus latest for variants that have it.
  if $updateTrack and any(.tags[]; split(":")[-1] | endswith("-stable" + $suffix)) then
    .tags += (
      ["\($dh):stable\($suffix)", "\($gh):stable\($suffix)"]
      + (if $addLatest then ["\($dh):latest", "\($gh):latest"] else [] end)
    )
  else . end |
  .tags | join(",")
' "$metadata_file"
