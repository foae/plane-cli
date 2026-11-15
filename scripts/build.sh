#!/bin/sh
# Only clean, version-matching tagged source receives a release identity.
set -eu
if [ "$#" -ne 1 ]; then
    printf '%s\n' 'usage: sh scripts/build.sh OUTPUT' >&2
    exit 2
fi
flags=''
if git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
    revision=$(git rev-parse HEAD)
    tag="v$(cat VERSION)"
    tagged_revision=$(git rev-parse --verify "refs/tags/$tag^{commit}" 2>/dev/null || true)
    if [ "$tagged_revision" = "$revision" ] && [ -z "$(git status --porcelain)" ]; then
        GOOS= GOARCH= go run ./internal/cmd/release version-check --tag "$tag"
        # version-check constrains the value before it enters Go linker flags.
        flags="-X main.version=$tag -X main.commit=$revision"
    fi
fi
go build -trimpath -ldflags "$flags" -o "$1" ./cmd/plane-cli
