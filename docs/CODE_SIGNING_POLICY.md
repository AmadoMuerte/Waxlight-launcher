# Code signing policy

Windows release artifacts are currently unsigned.

Official Waxlight release artifacts are built by GitHub Actions from tagged
commits in the public repository and are published through GitHub Releases.
The release workflow validates Windows executable metadata and provides
`SHA256SUMS` for integrity verification.

SHA-256 checksums verify artifact integrity against the release checksum but
do not provide publisher authentication.

Waxlight may introduce Windows code signing in the future. If that happens,
this policy and the release pipeline will be updated to describe the actual
signing mechanism.

Download Waxlight only from the official GitHub Releases page. Report security
issues related to release provenance, signing, or sensitive data using the
private process in [SECURITY.md](SECURITY.md).
