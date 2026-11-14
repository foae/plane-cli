#!/bin/sh
set -eu

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
pins="$root/tools/versions.mk"

fail() {
    printf '%s\n' "$*" >&2
    exit 1
}

# Parse a single literal assignment. Never source Make syntax or evaluate its RHS.
read_pin() {
    awk -v key="$1" '
        $1 == key {
            if (NF != 3 || $2 != ":=" || $3 !~ /^[A-Za-z0-9._-]+$/) bad = 1
            value = $3
            count++
        }
        END {
            if (bad || count != 1) exit 1
            print value
        }
    ' "$pins" || fail "Missing or invalid pin: $1"
}

go_version=$(read_pin GO_VERSION)
case "${1-}" in
    --print-go-version)
        [ "$#" -eq 1 ] || fail 'Usage: install-tools.sh [--print-go-version]'
        printf '%s\n' "$go_version"
        exit 0
        ;;
    '') [ "$#" -eq 0 ] || fail 'Usage: install-tools.sh [--print-go-version]' ;;
    *) fail 'Usage: install-tools.sh [--print-go-version]' ;;
esac

export GOTOOLCHAIN=local
export GOFLAGS=-mod=readonly
export GOSUMDB=sum.golang.org
for command in go curl tar awk; do
    command -v "$command" >/dev/null 2>&1 || fail "Required command not found: $command"
done
[ "$(go env GOVERSION)" = "go$go_version" ] || fail "Install Go $go_version and put it on PATH first."

case "$(uname -s)" in
    Linux) os=linux ;;
    Darwin) os=darwin ;;
    *) fail 'Tool installation supports Linux and macOS only.' ;;
esac
case "$(uname -m)" in
    x86_64|amd64) arch=amd64 ;;
    arm64|aarch64) arch=arm64 ;;
    *) fail 'Tool installation supports amd64 and arm64 only.' ;;
esac
lint_version=$(read_pin GOLANGCI_LINT_VERSION)
vuln_version=$(read_pin GOVULNCHECK_VERSION)
checksum=$(read_pin "GOLANGCI_LINT_SHA256_${os}_${arch}")
[ "${#checksum}" -eq 64 ] || fail 'Invalid golangci-lint SHA-256 length.'
case "$checksum" in *[!0-9a-f]*) fail 'Invalid golangci-lint SHA-256.' ;; esac

if command -v sha256sum >/dev/null 2>&1; then
    hash_command=sha256sum
elif command -v shasum >/dev/null 2>&1; then
    hash_command=shasum
else
    fail 'Install sha256sum or shasum first.'
fi

mkdir -p "$root/.bin"
staging=$(mktemp -d "$root/.bin/.install-tools.XXXXXX")
trap 'rm -rf "$staging"' 0
trap 'exit 1' HUP INT TERM
archive="golangci-lint-${lint_version}-${os}-${arch}.tar.gz"
curl --fail --location --silent --show-error --proto '=https' --proto-redir '=https' \
    "https://github.com/golangci/golangci-lint/releases/download/v${lint_version}/${archive}" \
    --output "$staging/$archive"
if [ "$hash_command" = sha256sum ]; then
    actual=$(sha256sum "$staging/$archive")
else
    actual=$(shasum -a 256 "$staging/$archive")
fi
actual=${actual%% *}
[ "$actual" = "$checksum" ] || fail "Checksum mismatch for $archive"
printf 'Verified SHA-256: %s  %s\n' "$actual" "$archive"

tar -xzf "$staging/$archive" -C "$staging" \
    "golangci-lint-${lint_version}-${os}-${arch}/golangci-lint"
GOBIN="$staging" go install "golang.org/x/vuln/cmd/govulncheck@$vuln_version"
mv "$staging/golangci-lint-${lint_version}-${os}-${arch}/golangci-lint" "$root/.bin/golangci-lint"
mv "$staging/govulncheck" "$root/.bin/govulncheck"
printf 'Installed golangci-lint %s and govulncheck %s in %s/.bin\n' "$lint_version" "$vuln_version" "$root"
cd "$root"
"$root/.bin/golangci-lint" config verify
