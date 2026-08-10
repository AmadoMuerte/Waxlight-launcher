---
title: Code signing
description: Windows release signing policy via the SignPath Foundation and artifact verification.
order: 30
---

# Code signing policy

The rules for signing Waxlight Windows release artifacts through the SignPath Foundation program.

***Free code signing provided by SignPath.io, certificate by SignPath Foundation.** Canonical source: [docs/CODE_SIGNING_POLICY.md](https://github.com/AmadoMuerte/Waxlight-launcher/blob/main/docs/CODE_SIGNING_POLICY.md).*

This policy applies to Windows release artifacts produced by the Waxlight Launcher project. Legacy releases published before SignPath Foundation signing is activated may be unsigned.

## Project and source repository

- Project: **Waxlight Launcher**
- Repository: [github.com/AmadoMuerte/Waxlight-launcher](https://github.com/AmadoMuerte/Waxlight-launcher)
- License: GNU GPL v3.0
- Maintainer: [AmadoMuerte](https://github.com/AmadoMuerte)

Only artifacts built from source code and build scripts controlled in this repository are eligible for signing through the Waxlight SignPath subscription. Waxlight will not use its subscription to sign unrelated projects, private/proprietary components, or upstream binaries presented as Waxlight binaries.

## Roles

| Role | Who |
| --- | --- |
| Authors / committers | AmadoMuerte |
| Reviewers | AmadoMuerte — reviews changes from contributors without commit access |
| Approvers | AmadoMuerte — manually reviews and approves every SignPath signing request |

The current one-person arrangement does not provide independent review or approval. SignPath's published terms require these roles but do not explicitly require different people. If additional trusted maintainers receive these roles, this document will be updated. Before using SignPath access, the maintainer manually verifies multi-factor authentication for every repository and SignPath team member.

## Review and approval rules

- Contributions from users without commit access are merged only after maintainer review.
- Release build scripts, GitHub Actions workflows, packaging definitions, dependency changes, and signing configuration receive the same review attention as application code.
- Every SignPath signing request requires manual approval by an Approver.
- Signing is tied to a verifiable automated build from the public source repository; locally produced or manually substituted binaries are not eligible.

## Build provenance

Windows release builds originate from a tagged commit in the public repository. GitHub Actions checks out that tag on a Windows runner, builds the artifacts, validates their metadata, and uploads them as CI artifacts. The publish job downloads those artifacts, generates `SHA256SUMS`, and attaches the final files to the GitHub release. After SignPath approval, only these eligible CI artifacts are submitted in a signing request.

## Artifact rules

The intended signing coverage is:

1. the Waxlight application executable before it is placed in a portable archive or installer;
2. the Windows installer after packaging;
3. any additional Waxlight-owned executable introduced in the future only after it is added to the reviewed artifact configuration.

Unsigned upstream components may be included where permitted by the SignPath Foundation policy, but Waxlight will not sign upstream binaries as if they were Waxlight binaries.

## File metadata restrictions

The SignPath artifact configuration enforces Windows file metadata on signed binaries: product-name attributes must identify **Waxlight Launcher**, and file/product version attributes within a release use the release's consistent Windows version value. The release build validates the executable, the installer, and the executable inside the portable archive before publishing; the post-approval pipeline additionally verifies Authenticode on each signed artifact.

Windows prereleases use `alpha.N`, `beta.N`, or `rc.N`; the numeric Windows revision reserves separate ranges for those channels, so `beta.1` and `beta.2` cannot share a Windows file version.

## Verifying as a user

> [!NOTE] Always verify the specific file
> A release being listed on GitHub does not by itself prove Authenticode signing. Verify the signature of the specific downloaded artifact. Releases predating SignPath activation may legitimately be unsigned.

On the difference between SHA-256 and Authenticode, see [Launcher updates](../features/updates.md). Report security issues related to release provenance or signing privately via the [security policy](./security.md) process.
