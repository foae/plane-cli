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

Keep the formula's local fake server self-contained in its Ruby test block using standard-library `TCPServer` and a child process; no Python, Go, WEBrick gem or test-only runtime dependency. Bind an ephemeral port, synchronize readiness without sleeps, serve a reviewed project-list fixture with exact route checks, invoke the installed binary with isolated config/external auth, and assert decoded JSON. Bound the child lifetime and always reap it. Offline version is a separate assertion.

Because the user selected mock-based acceptance, real keyring/Plane-server proof is recorded as unverified where not exercised, not treated as passed or a hidden release blocker.

## Workflow and provenance

`.github/workflows/release.yml` prepares artifacts only for approved tagged commits after required checks, with least-privilege permissions, SHA-pinned actions and no secrets for untrusted pull requests. Keep publishing behind an explicit approval/environment. Do not auto-create tags, push formula changes or upload artifacts while implementing/testing the release packet.

Sequence release inputs explicitly: build immutable archives, calculate checksums, generate the four-target formula from those exact artifacts, then test a temporary formula copy against staged local HTTP assets. Retain the production URL template in the maintained formula, not a CI hostname. After authorized publication, verify each published asset against the recorded checksum before proposing the corresponding formula update to the maintainer-selected tap. Never publish a formula that refers to unavailable assets, or claim a staged install proves publication.

Record CLI version/commit, toolchain/dependency pins, source catalog hashes, target matrix, checksums and exact acceptance results. Distinguish upstream documentation snapshot, Community release tag, Commercial release version and CLI version; they are not interchangeable.

Follow SemVer for the CLI's envelope, command/flag and exit-code contract. Additive commands/fields are minor changes; incompatible behavior needs a major and migration instructions. Document upstream API behavior changes without silently changing the selected generation. Release notes contain dated change history; standing architecture/auth docs contain current policy only.
