#!/bin/sh
# Local acceptance only: never install into the caller's GOBIN or Homebrew prefix.
set -eu
if [ "$#" -ne 1 ]; then
  echo 'usage: sh scripts/check-installation.sh ARTIFACT_DIR' >&2
  exit 2
fi
artifacts=$(cd "$1" && pwd -P)
root=$(pwd -P)
brew=${BREW:-$(command -v brew || true)}
if [ -z "$brew" ] && [ -x /home/linuxbrew/.linuxbrew/bin/brew ]; then
  brew=/home/linuxbrew/.linuxbrew/bin/brew
fi
if [ -z "$brew" ] || [ ! -x "$brew" ]; then
  echo 'installation-check requires installed Homebrew; set BREW to its executable' >&2
  exit 1
fi
# Homebrew reads this even with env -i and lets it override cache/temp settings.
# Refuse that configuration rather than risk writing outside the sandbox.
if [ -r /etc/homebrew/brew.env ]; then
  echo 'installation-check requires a host without /etc/homebrew/brew.env to guarantee isolation' >&2
  exit 1
fi
# --repository is a read-only shell query; no command runs against the installed
# Homebrew after this. All mutable runtime files are copied, never symlinked.
repository=$(HOMEBREW_NO_AUTO_UPDATE=1 HOMEBREW_NO_ANALYTICS=1 "$brew" --repository)
if [ ! -x "$repository/Library/Homebrew/vendor/portable-ruby/current/bin/ruby" ]; then
  echo 'installation-check requires Homebrew portable Ruby already installed' >&2
  exit 1
fi
# Homebrew's Linux sandbox grants all of /tmp and /var/tmp writable. Keep its
# executable outside those trees so the inherited-sandbox protection probe passes.
tmp=$(mktemp -d "${HOME:?}/.plane-installation.XXXXXXXX")
trap 'rm -rf "$tmp"' EXIT
trap 'exit 130' INT
trap 'exit 143' HUP TERM
mkdir -p "$tmp/prefix/bin" "$tmp/home" "$tmp/cache" "$tmp/logs" "$tmp/temp" "$tmp/gobin"
git clone --quiet --shared --no-hardlinks "$repository" "$tmp/prefix/Homebrew"
ln -s ../Homebrew/bin/brew "$tmp/prefix/bin/brew"
for directory in portable-ruby bundle gems; do
  if [ -d "$repository/Library/Homebrew/vendor/$directory" ]; then
    cp -R "$repository/Library/Homebrew/vendor/$directory" "$tmp/prefix/Homebrew/Library/Homebrew/vendor/"
  fi
done
printf '%s\n' 'Installing source CLI into isolated GOBIN'
GOBIN="$tmp/gobin" GOCACHE="$tmp/go-build" GOMODCACHE="$tmp/go-mod" GOWORK=off GOENV=off \
  go install -modcacherw ./cmd/plane-cli
for argument in version --help; do
  PLANE_CLI_CONFIG_DIR="$tmp/go-config" "$tmp/gobin/plane-cli" "$argument" > "$tmp/go-$argument.out"
done
for generation in v1 v2; do
  PLANE_CLI_CONFIG_DIR="$tmp/go-config" "$tmp/gobin/plane-cli" commands schema --api-version "$generation" > "$tmp/go-schema-$generation.out"
done
host_os=$(go env GOHOSTOS)
host_arch=$(go env GOHOSTARCH)
# Start from an empty environment: inherited Homebrew paths, API credentials,
# user configuration and proxies cannot redirect this isolated installation.
env -i HOME="$tmp/home" USER="$(id -un)" \
  PATH="$tmp/prefix/bin:/usr/bin:/bin:/usr/sbin:/sbin" \
  TMPDIR="$tmp/temp" HOMEBREW_TEMP="$tmp/temp" \
  HOMEBREW_CACHE="$tmp/cache" HOMEBREW_LOGS="$tmp/logs" \
  HOMEBREW_NO_AUTO_UPDATE=1 HOMEBREW_NO_ANALYTICS=1 HOMEBREW_NO_ENV_HINTS=1 \
  HOMEBREW_NO_INSTALL_CLEANUP=1 HOMEBREW_NO_INSTALL_FROM_API=1 \
  HOMEBREW_NO_GITHUB_API=1 HOMEBREW_FORCE_VENDOR_RUBY=1 HOMEBREW_DEVELOPER=1 \
  "$tmp/prefix/Homebrew/Library/Homebrew/vendor/portable-ruby/current/bin/ruby" \
  "$root/scripts/check-formula.rb" "$artifacts" "$tmp" "$host_os" "$host_arch"
