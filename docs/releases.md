# Release and installation plan

No release publication is claimed here. P10a provides version planning and preparation; artifact packaging/publication remains planned. Publishing tags/releases and external Homebrew tap changes need explicit maintainer authorization. The ordered delivery slices are in [P10](implementation/10-delivery.md).

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

## Version policy and tooling

The first full stable release is `v1.0.0`, after the final coverage and delivery gates pass. Unresolved required operations, including bounded packet implementations awaiting planner review, prevent that claim. Authorized earlier previews use `v1.0.0-alpha.N`, `v1.0.0-beta.N` or `v1.0.0-rc.N`, explicitly marked prerelease; do not silently redefine stable scope to ship early.

Every new work item carries a reviewed release-impact entry: `patch` for compatible fixes, security/dependency corrections or documentation/skill corrections; `minor` for backward-compatible functionality; `major` for breaking public CLI or skill-consumer contracts. Internal-only work still receives an explicit patch entry. SemVer is assessed by consumer behavior, not commit-message prefixes or packet numbers. A feature packet is not automatically a major version.

Development-only Go tooling under `internal/cmd/release/` is exposed through Make, without hooks or a second task runner:

- `make change LEVEL=patch|minor|major SUMMARY='…'` creates one uniquely named JSON fragment under `changes/`, with `level` and `summary` fields. Each work item includes its fragment. CI checks new entries against the PR base.
- `make version-plan` is read-only: validate `VERSION`, fragments and release history, then show the highest required bump and pending notes. Several changes produce one bump. Before the first stable release, retain the `1.0.0` target and increment the explicitly prepared prerelease sequence.
- `make release-prepare VERSION=X.Y.Z[-prerelease]` validates the requested version, updates `VERSION`, writes structured release notes to `releases/<version>.json`, and consumes included fragments. It requires a clean tree, rejects existing tags and insufficient bumps, and neither commits, tags nor publishes. Stable preparation additionally runs the full API coverage gate. Notes retain version, UTC date and individual change entries. Supported preview channels are `alpha.N`, `beta.N` and `rc.N`.
- `make version-check` validates version and release history. `BASE=<commit>` additionally checks release impact across a PR: ordinary work adds new fragments without modifying or deleting existing fragments, history or `VERSION`. CI selects explicit release-preparation mode when an existing `VERSION` changes; this mode validates the consumption and rejects unrelated changes rather than requiring a recursive fragment. `VERSION` is the release-target source of truth, not the toolchain pin file.

A prepared preview may advance to a later preview channel or stable on the same core version without a new fragment. Its notes record `promoted_from` with the exact prior preview version and an empty `changes` array; follow that lineage to the original change entries. This does not permit empty same-channel sequence bumps or new releases from stable versions. Preparation with pending fragments records those changes directly.

`make build` and `make cross` inject release identity only for clean source where the exact `v<VERSION>` tag identifies HEAD; unrelated tags do not prevent development builds. Other source builds report development identity, including direct `go build` from tagged checkouts. Tagged-module `go install` retains module-version fallback only without local VCS metadata, excluding replaced modules and commit pseudo-versions. These commands do not create tags. The initial `1.0.0-alpha.0` target is not a published release.

P10b can validate its generated packaged metadata with `go run ./internal/cmd/release version-check --skill-metadata <file>`: the JSON `skill_version` and `cli_version` must both match `VERSION`. No skill artifact is claimed by P10a.

Mutating tooling takes a fail-fast lock in the worktree's Git directory. After an interrupted process, remove the lock named in the error only after confirming no release process remains. Preparation requires tracked release inputs; ignored build artifacts are allowed, but ignored fragments are not consumed.

The skill shares the CLI release version and tag; there is no independent skill release counter. Skill-only work therefore prepares at least a patch release. JSON `schema_version`, Plane API generation and upstream product versions remain independent.

Acceptance: deterministic fixtures for patch/minor/major precedence, prerelease ordering and promotion, invalid/duplicate versions, concurrent fragment names, failed preparation without partial changes, and build/tag/version mismatch. Exercise the actual Make commands in a temporary repository; never create a real project tag during acceptance.

## GitHub publication and Homebrew — planned P10c

Use annotated immutable `vX.Y.Z` tags (including prerelease suffixes) on reviewed commits. An explicitly authorized maintainer creates/pushes the tag; tooling must not force-move it. The release workflow verifies the tag against `VERSION` and required checks, builds from that exact commit, and stages a draft GitHub Release before approval makes it public. Stable releases are not prereleases; preview tags never become GitHub's stable latest release.

Asset names are `plane-cli_X.Y.Z_OS_ARCH.tar.gz` for the four supported targets, `plane-cli-skill_X.Y.Z.tar.gz`, and `SHA256SUMS`; retain any prerelease suffix in `X.Y.Z`. Include the version/commit and compatibility/coverage limitations in release notes. Record hashes for every archive, including the skill. Refuse replacement of differing assets on an existing release; a failed publication resumes only against the same tag and verified bytes.

Homebrew is a maintainer-selected tap, not an assumed Homebrew/core submission. The release job generates the formula from the published stable assets and proposes the authorized tap update only after their checksum verification. Prereleases do not replace the stable formula. Formula/version/URLs/checksums must agree; test both target selection and the installed CLI as described above. Tap ownership/access remains a publication prerequisite, not a reason to skip local formula acceptance.

## Explicit update checking — planned P10d

Add `plane-cli update check`, a separate local-management command, not the Plane `release` resource. It is user-invoked network access only: ordinary commands, help, completion, `version` and startup never check or warn about updates. No background jobs, telemetry, persistent cache, binary replacement, shell execution or automatic skill installation.

Query the public GitHub Releases API for `foae/plane-cli` over verified HTTPS, without Plane configuration, PAT/OAuth/keyring lookup or forwarding any Plane credentials. Use a dedicated bounded HTTP path, normal proxy trust and explicit timeout/cancellation; reject redirects rather than inheriting Plane/storage redirect exceptions. No production endpoint override flag; tests inject a fake server internally. Default selection is the latest published stable SemVer release; `--include-prerelease` includes published prereleases using bounded pagination and reports a protocol error rather than claiming latest if the scan bound is exceeded. Ignore drafts/non-SemVer tags and compare numerically, not lexically.

Return the existing JSON success envelope with `data.current_version`, `latest_version` (nullable), `update_available` (nullable for unknown development builds), `channel`, `release_url` (nullable), and `status` (`up_to_date`, `update_available`, `no_release` or `unknown_current`). An available update is exit 0, not an error. A development build never guesses its installed SemVer. No releases is an explicit successful state; rate limits, network failure and malformed metadata are errors using the existing exit contract, never “up to date.” Validate release URLs against the expected HTTPS GitHub repository; never print arbitrary server-provided upgrade instructions.

Document manual upgrade paths: Homebrew users use their tap's `brew upgrade plane-cli`; archive users verify and replace explicitly; `go install` users select the exact tag. The matching versioned skill is a separate intentional install. No install-method autodetection is needed for a check.

Acceptance: fake-server stable/prerelease ordering, older/equal/newer current versions, development builds, no releases, drafts, pagination bounds, invalid metadata, rate limiting, cancellation and cross-origin redirects; assert zero Plane credential access and no local writes. Process-level offline help/version tests must prove zero update traffic.
