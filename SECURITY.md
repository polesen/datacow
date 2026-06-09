# Security Policy

## Reporting a vulnerability

Please **do not report security vulnerabilities through public GitHub issues.**

Instead, use GitHub's private vulnerability reporting:

1. Go to the **Security** tab of this repository.
2. Click **Report a vulnerability**.

This opens a private advisory visible only to the maintainers. We aim to
acknowledge reports within a few days.

If private reporting is unavailable, email the maintainer at the address on
their GitHub profile.

## Scope

Datacow is a **read-only** database explorer. The most security-sensitive area
is SQL handling — the project's hard rule is that all queries are parameterized
and dynamic identifiers are validated against the known schema (see the "SQL
Security" section of `CLAUDE.md`). Reports of SQL injection vectors, credential
handling issues, or supply-chain concerns are especially valued.

## Supply chain

Release binaries are built by GitHub Actions via GoReleaser. Each release
publishes a `checksums.txt` (SHA256) file and a signed SLSA build-provenance
attestation. You can verify a downloaded artifact with:

```bash
gh attestation verify <artifact> --repo polesen/datacow
```
