#!/bin/sh
set -eu
if [ "$#" -ne 1 ]; then
  echo 'usage: sh scripts/check-packaged-skill.sh ARTIFACT_DIR' >&2
  exit 2
fi
artifacts=$(cd "$1" && pwd -P)
version=$(cat VERSION)
os=$(go env GOHOSTOS)
arch=$(go env GOHOSTARCH)
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM
mkdir "$tmp/cli" "$tmp/skill"
tar -xzf "$artifacts/plane-cli_${version}_${os}_${arch}.tar.gz" -C "$tmp/cli"
tar -xzf "$artifacts/plane-cli-skill_${version}.tar.gz" -C "$tmp/skill"
PLANE_CLI_SKILL_DIR="$tmp/skill/plane-cli" PLANE_CLI_SKILL_BINARY="$tmp/cli/plane-cli" \
  go test ./internal/cli -run '^TestPackagedSkill$' -count=1
