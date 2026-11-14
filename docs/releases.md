# Release and installation plan

No releases exist in this repository yet. P10 implements these requirements; publishing and external Homebrew tap changes need explicit maintainer authorization.

## Supported artifacts

Four CGO-free binaries: `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`, all named `plane-cli` inside their archive. Build with `-trimpath`, controlled ldflags for version/commit/date, and explicit GOOS/GOARCH. Build metadata falls back to Go's `debug.ReadBuildInfo` for `go install`; untagged builds say development, never impersonate a release.

Use versioned `.tar.gz` archives containing binary, LICENSE and brief installation guidance; include SHA-256 checksums. Deterministic input ordering/timestamps and controlled release metadata should make repeated builds from the same inputs reproducible; verify rather than claim. `make snapshot` produces local nonpublishing artifacts. A small release script is enough; do not add GoReleaser unless its concrete benefit outweighs another pinned tool.

Installation paths:

- Download an archive, verify its checksum, extract and put `plane-cli` on PATH. No shell installer or implicit sudo.
- `go install github.com/foae/plane-cli/cmd/plane-cli@vX.Y.Z` for tagged releases; source build documentation states the Go floor.
- Homebrew formula maintained at `Formula/plane-cli.rb`, selecting the correct Linux/macOS architecture archive and checksum; test invokes offline `version` and a local fake-server scenario, not production Plane. A maintainer-owned tap/distribution destination must be set explicitly before publishing; no assumed ownership of a remote repository.

## Artifact verification

Cross-compilation proves target builds, not native runtime compatibility. Run the fake-server CLI suite natively on available Linux/macOS CI runners; record architecture evidence honestly. At minimum verify each archive contains the intended target binary using binary headers, correct executable permissions, LICENSE and checksum entry. Never execute a foreign binary and call that native smoke.

For the host, unpack into a fresh temp prefix and exercise `version`, offline help/schema and a create/read/update/delete fake-server lifecycle. Check the `go install` path using an isolated GOBIN/module cache as appropriate, without overwriting the user's installation. Validate the formula syntax/audit and installation against staged snapshot assets on controlled CI; absence of a real published tap is a publication step, not fabricated install evidence.

Because the user selected mock-based acceptance, real keyring/Plane-server proof is recorded as unverified where not exercised, not treated as passed or a hidden release blocker.

## Workflow and provenance

`.github/workflows/release.yml` prepares artifacts only for approved tagged commits after required checks, with least-privilege permissions, SHA-pinned actions and no secrets for untrusted pull requests. Keep publishing behind an explicit approval/environment. Do not auto-create tags, push formula changes or upload artifacts while implementing/testing the release packet.

Record CLI version/commit, toolchain/dependency pins, source catalog hashes, target matrix, checksums and exact acceptance results. Distinguish upstream documentation snapshot, Community release tag, Commercial release version and CLI version; they are not interchangeable.

Follow SemVer for the CLI's envelope, command/flag and exit-code contract. Additive commands/fields are minor changes; incompatible behavior needs a major and migration instructions. Document upstream API behavior changes without silently changing the selected generation. Release notes contain dated change history; standing architecture/auth docs contain current policy only.
