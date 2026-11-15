#!/bin/sh
# Invoked only by the explicitly approved publication workflow.
set -eu
if [ "$#" -ne 2 ]; then
  echo 'usage: sh scripts/publish-release.sh ARTIFACT_DIR draft|publish' >&2
  exit 2
fi
case "$2" in draft|publish) ;; *) exit 2 ;; esac
artifacts=$(cd "$1" && pwd -P)
root=$(pwd -P)
version=$(cat VERSION)
tag="v$version"
repository=foae/plane-cli
scratch=$(mktemp -d)
trap 'rm -rf "$scratch"' EXIT HUP INT TERM
fail() { echo "$*" >&2; exit 1; }
test -z "$(git status --porcelain)" || fail 'checkout is dirty'
test "$(git cat-file -t "refs/tags/$tag")" = tag || fail 'release tag must be annotated'
commit=$(git rev-parse HEAD)
test "$(git rev-parse "refs/tags/$tag^{commit}")" = "$commit" || fail 'tag does not match HEAD'
git ls-remote --tags origin "refs/tags/$tag" "refs/tags/$tag^{}" > "$scratch/remote-tag"
test "$(awk -v ref="refs/tags/$tag" '$2 == ref { print $1 }' "$scratch/remote-tag")" = "$(git rev-parse "refs/tags/$tag")" || fail 'remote annotated tag differs'
test "$(awk -v ref="refs/tags/$tag^{}" '$2 == ref { print $1 }' "$scratch/remote-tag")" = "$commit" || fail 'remote peeled tag differs'
go run ./internal/cmd/release version-check --tag "$tag"
# Parse and verify only explicit local filenames, never manifest-controlled paths.
python3 - "$root" "$artifacts" "$version" "$commit" > "$scratch/expected" <<'PY'
import hashlib, json, pathlib, re, sys
root, directory, version, commit = sys.argv[1:]
root, directory = pathlib.Path(root), pathlib.Path(directory)
def require(ok, message):
    if not ok:
        raise SystemExit(message)
require(re.fullmatch(r'(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-(alpha|beta|rc)\.(0|[1-9][0-9]*))?', version), 'invalid VERSION')
targets = ['linux/amd64', 'linux/arm64', 'darwin/amd64', 'darwin/arm64']
archives = [
    f'plane-cli_{version}_linux_amd64.tar.gz',
    f'plane-cli_{version}_linux_arm64.tar.gz',
    f'plane-cli_{version}_darwin_amd64.tar.gz',
    f'plane-cli_{version}_darwin_arm64.tar.gz',
    f'plane-cli-skill_{version}.tar.gz',
]
hashed = archives + ['plane-cli.rb', 'release-metadata.json']
expected = hashed + ['SHA256SUMS']
for name in expected:
    path = directory / name
    require(path.is_file() and not path.is_symlink(), f'missing or nonregular asset: {name}')
metadata = json.loads((directory / 'release-metadata.json').read_text())
require(all(metadata.get(key) == value for key, value in [('version', version), ('cli_version', version), ('commit', commit)]), 'metadata identity mismatch')
require(metadata.get('snapshot') is False and metadata.get('source_dirty') is False, 'metadata is not a clean release')
require(metadata.get('targets') == targets, 'metadata targets mismatch')
digest = lambda path: hashlib.sha256(path.read_bytes()).hexdigest()
inputs = ['api/catalog.json', 'api/provenance.json', 'go.mod', 'go.sum', 'tools/versions.mk']
require(metadata.get('input_sha256') == {name: digest(root / name) for name in inputs}, 'metadata input hashes mismatch')
require(metadata.get('archive_sha256') == {name: digest(directory / name) for name in archives}, 'metadata archive hashes mismatch')
lines = (directory / 'SHA256SUMS').read_text().splitlines()
manifest = {}
for line in lines:
    match = re.fullmatch(r'([0-9a-f]{64})  ([A-Za-z0-9_.-]+)', line)
    require(match is not None, 'invalid checksum manifest entry')
    checksum, name = match.groups()
    require(name in hashed and name not in manifest, 'unexpected or duplicate checksum filename')
    manifest[name] = checksum
require(manifest == {name: digest(directory / name) for name in hashed}, 'checksum manifest mismatch')
print('\n'.join(expected))
PY
cd "$artifacts"
# Listing distinguishes an absent release from authentication/network failure.
gh api --paginate "repos/$repository/releases?per_page=100" --jq '.[] | select(.tag_name == "'"$tag"'") | .id' > "$scratch/ids"
exists=false
if [ -s "$scratch/ids" ]; then
  exists=true
  gh release view "$tag" --repo "$repository" --json assets,isDraft --jq '.assets[].name' > "$scratch/names"
  draft=$(gh release view "$tag" --repo "$repository" --json isDraft --jq .isDraft)
  # Complete the read-only comparison before any upload or edit.
  while IFS= read -r name; do
    grep -Fqx "$name" "$scratch/expected" || fail "unexpected remote asset: $name"
    gh release download "$tag" --repo "$repository" --pattern "$name" --dir "$scratch"
    cmp "$name" "$scratch/$name" || fail "remote asset differs: $name"
  done < "$scratch/names"
  while IFS= read -r name; do
    if ! grep -Fqx "$name" "$scratch/names"; then
      test "$draft" = true || fail "refusing to add assets to public release: $name"
    fi
  done < "$scratch/expected"
else
  test "$2" = draft || fail 'draft release does not exist'
  : > "$scratch/names"
fi
if [ "$exists" = false ]; then
  preview=false
  case "$version" in *-*) preview=true ;; esac
  gh release create "$tag" --repo "$repository" --verify-tag --draft --prerelease="$preview" --latest=false \
    --title "$tag" --notes "Documentation-backed CLI release. See release-metadata.json for exact source, toolchain and compatibility limitations; live Plane editions and native keyrings are unverified."
fi
while IFS= read -r name; do
  if ! grep -Fqx "$name" "$scratch/names"; then
    gh release upload "$tag" "$name" --repo "$repository"
    gh release download "$tag" --repo "$repository" --pattern "$name" --dir "$scratch"
    cmp "$name" "$scratch/$name"
  fi
done < "$scratch/expected"
if [ "$2" = publish ]; then
  case "$version" in
    *-*) gh release edit "$tag" --repo "$repository" --draft=false --latest=false ;;
    *) gh release edit "$tag" --repo "$repository" --draft=false ;;
  esac
fi
