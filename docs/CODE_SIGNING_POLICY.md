# Code signing policy

**Free code signing provided by [SignPath.io](https://about.signpath.io), certificate by [SignPath Foundation](https://signpath.org)**

This policy applies to Windows release artifacts produced by the Waxlight Launcher project. Legacy releases published before SignPath Foundation signing is activated may be unsigned.

## Project and source repository

- Project: **Waxlight Launcher**
- Source repository: https://github.com/AmadoMuerte/Waxlight-launcher
- License: GNU General Public License v3.0
- Maintainer: [AmadoMuerte](https://github.com/AmadoMuerte)

Only artifacts built from source code and build scripts controlled in this repository are eligible to be signed through the Waxlight SignPath subscription. Waxlight will not use its signing subscription to sign unrelated projects, private/proprietary components, or upstream binaries presented as Waxlight binaries.

## Code signing roles

Waxlight is currently maintained as an individual open-source project. The current roles are:

- **Authors / Committers:** [AmadoMuerte](https://github.com/AmadoMuerte)
- **Reviewers:** [AmadoMuerte](https://github.com/AmadoMuerte) — reviews changes proposed by contributors who do not have commit access before they are merged.
- **Approvers:** [AmadoMuerte](https://github.com/AmadoMuerte) — manually reviews and approves every SignPath signing request.

The current one-person arrangement does not provide independent review or approval. SignPath's published terms require these roles but do not explicitly require different people for them. If additional trusted maintainers receive these roles, this document will be updated. Before applying for or using SignPath access, the maintainer must manually verify multi-factor authentication for every repository and SignPath team member.

## Review and approval rules

- Contributions from users without commit access are merged only after maintainer review.
- Release build scripts, GitHub Actions workflows, packaging definitions, dependency changes, and signing configuration receive the same review attention as application code.
- Every SignPath signing request requires manual approval by an Approver.
- Signing must be tied to a verifiable automated build from this public source repository. Locally produced or manually substituted binaries are not eligible for signing.

## Artifact rules

Waxlight's SignPath configuration will be restricted to Waxlight Windows release artifacts produced by the automated release build. The intended signing coverage is:

1. the Waxlight application executable before it is placed in a portable archive or installer;
2. the Windows installer after packaging;
3. any additional Waxlight-owned executable introduced in the future only after it is added to the reviewed artifact configuration.

Unsigned upstream open-source/system components may be included where permitted by the SignPath Foundation policy, but Waxlight will not sign upstream binaries as if they were Waxlight binaries.

## File metadata restrictions

SignPath artifact configuration will enforce Windows file metadata for signed Waxlight binaries. Product-name attributes must identify **Waxlight Launcher**, and file/product version attributes within a release build must use the release's consistent Windows version value.

The release build validates the application executable, the installer, and the executable contained in the portable archive before publishing. The post-approval pipeline must additionally verify Authenticode on each signed artifact before publication.

Windows prereleases use `alpha.N`, `beta.N`, or `rc.N`. The numeric Windows revision reserves separate ranges for those channels, so distinct prereleases such as `beta.1` and `beta.2` cannot receive the same Windows file version.

## Privacy

Waxlight's [Privacy Policy](PRIVACY.md) describes local data, optional telemetry, and affected third-party services. Optional telemetry is disabled by default for fresh installations; the Windows installer displays the policy and provides a separate opt-in control, and the preference can later be changed in the application.

Relevant third-party privacy policies for network services used by Waxlight include:

- Vintage Story / Anego Studios: https://www.vintagestory.at/privacy/
- GitHub: https://docs.github.com/site-policy/privacy-policies/github-privacy-statement

## Release-page disclosure

Waxlight download and GitHub release pages link to this **Code signing policy**. A release being listed on GitHub does not by itself prove that an executable is Authenticode-signed. Users should verify the signature of the specific Windows artifact. Releases predating SignPath activation may legitimately be unsigned.

## Security reports

Security issues related to release provenance, signing, or sensitive data should be reported using the private process in [SECURITY.md](SECURITY.md).
