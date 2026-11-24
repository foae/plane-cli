# Authoritative development tool pins; runtime dependencies belong in go.mod.
# Keep assignments literal: scripts/install-tools.sh parses data, never sources it.
GO_VERSION := 1.27.4
GOLANGCI_LINT_VERSION := 2.15.0
GOVULNCHECK_VERSION := v1.10.0

# Reviewed upstream release checksum data:
# https://github.com/golangci/golangci-lint/releases/download/v2.15.0/golangci-lint-2.15.0-checksums.txt
GOLANGCI_LINT_SHA256_linux_amd64 := 9b7fe6a6bf6334b98decf8d86b11415ea7889afdabdd1959fdce23af2a9ebcea
GOLANGCI_LINT_SHA256_linux_arm64 := b4d8f17068b35ee6060cc2f69a16fb48740d706fbba81661c318649c114a6998
GOLANGCI_LINT_SHA256_darwin_amd64 := 767b899a28285e88ded2823d5dcc4bc5ef681173eb2d80f029a7b17ee4ece3b8
GOLANGCI_LINT_SHA256_darwin_arm64 := d1e409481a8ca846c4bc97e781eb5cb1bd5c23353a0b3977a1cbda1b1b6447d5
