# Release integrity and reproducibility

## Installation

The installers default to the `v0.1.0-alpha.8` tag. This is explicit
because GitHub's `latest` download endpoint excludes prereleases. Override
`BYECLAUDE_VERSION` to select another published tag; use `latest` only when a
stable release exists. Alpha.4 adds local metrics and optional, recoverable
user PATH setup. Go is only needed to build from source;
release binaries need Git, with no admin rights or runtime installation.

On Windows, download and inspect `install.ps1`, then run:

```powershell
powershell -NoProfile -File .\install.ps1
```

The installer adds the install directory to your **user** PATH by default,
without administrator rights, UAC or changes to system PATH. `-AddToPath` is
still accepted for compatibility; `-NoPath` opts out. Reopen cmd/PowerShell
afterwards for PATH in other commands. In an interactive terminal, installation opens the guide immediately using the full installed path; repository changes still require confirmation. `-NoStart` or `BYECLAUDE_NO_START=1` disables automatic launch. CI and redirected input/output never launch it. `BYECLAUDE_INSTALL_DIR` selects a preferred directory; if staging
there is impossible, the installer tries `%LOCALAPPDATA%\Programs\ByeClaude`,
then `%USERPROFILE%\.local\bin\ByeClaude`. It reports the actual destination.

On Unix, the default is `~/.local/bin`; an unwritable custom directory falls
back there without sudo. PATH setup maintains a marked block in Bash's
`.bashrc` and active login profile, Zsh's `.zshrc` (respecting absolute
`ZDOTDIR`), or sh's `.profile`. Linked, ambiguous or oversized profiles and
unsupported shells are left for manual setup. `BYECLAUDE_NO_PATH=1` opts out. The shell installer also opens guided setup in an interactive terminal; `--no-start` or `BYECLAUDE_NO_START=1` disables it. Git must already be installed.

PATH failure is nonfatal once the binary is installed. The full executable
path is printed, and a saved pending preference offers Retry / Later / No at
the next interactive start. `byeclaude path status`, `path setup` and `path
skip` inspect, retry or disable reminders. No prompt appears in scripts or
Git hooks. If the retry preference itself cannot be saved, the diagnostic
explicitly tells you to retry manually. Neither setup nor the application
requests elevation; declining a separate administrator prompt is therefore
unnecessary for normal installation.

Failed integrity checks still stop installation. Existing binaries survive a
failed checksum verification. If no writable directory exists, or an existing
binary is locked against replacement, the installer reports that genuine
failure rather than claiming a successful installation.

For development from a checkout, build `go build -trimpath -o
dist/byeclaude.exe ./cmd/byeclaude` on Windows (omit `.exe` on Unix). Keep that
binary at a persistent path before installing Git hooks.

## Release artifacts

CI runs on the public [angusu-de mirror](https://github.com/angusu-de/ByeClaude/actions/workflows/ci.yml),
using the same source commit as the canonical repository. Its
[live evidence](https://github.com/angusu-de/ByeClaude/blob/ci-proof/proof/README.md)
records the exact commit, workflow attempt, job outcomes and individual steps.
Missing, skipped and failed jobs never count as passed. The separate account
belongs to the same maintainer; this is reproducibility evidence, not an
independent audit. The SVG design is exported from `IamAngusU/Badges` at
`775ec50cbc778fc4e4181ba14b98f9fc0beb4e5c` and vendored here so CI needs no private-repository token.

When release builds run on that mirror, the verified release set is promoted
unchanged to the canonical repository at the identical tag and source commit.
Use the canonical release's build-evidence link to inspect the actual run.

ByeClaude release tags pass the same rewrite, fixture, race, security, installer
and attribution gates used for normal changes. A release publishes six static
binaries: Linux, macOS and Windows on amd64 and arm64.

The release set also contains:

- `SHA256SUMS.txt`, covering every binary, the SBOM and the provenance record;
- `SBOM.cdx.json`, a CycloneDX 1.5 inventory read from the linked Go build;
- `BUILD-PROVENANCE.json`, binding artifact hashes to the repository, exact
  source commit, source timestamp, release version and Go toolchain.

The release workflow also creates a GitHub artifact attestation for every file
in that set, using GitHub's OIDC identity and Sigstore-backed transparency log,
and verifies every attestation before publishing the release. Because release
artifacts are built on the public mirror and promoted unchanged, verify the
builder identity against that repository:

```sh
gh attestation verify byeclaude_windows_amd64.exe --repo angusu-de/ByeClaude
```

The installers download a binary and the checksum file, require an exact
matching asset entry, verify SHA-256 before installation, execute the staged
binary, and replace an existing installation only after those checks pass.

## Reproducibility

`scripts/test-release-reproducibility.ps1` builds every supported target twice
with `-trimpath`, disabled CGO, disabled implicit VCS stamping and an explicit
release version. It then generates the SBOM, provenance and checksums twice and
requires the two complete release sets to be byte-identical.

Run it with the Go version declared in `go.mod`:

```powershell
pwsh -NoProfile -File scripts/test-release-reproducibility.ps1
```

## Trust boundary

`BUILD-PROVENANCE.json` deliberately remains a deterministic, unsigned local
build record so the complete release set can be reproduced byte-for-byte.
The separate GitHub artifact attestation cryptographically binds each released
file to the release workflow identity and records that statement in Sigstore's
transparency log. Checksums protect the installer download; the attestation is
the independently verifiable provenance layer. Both ultimately trust the
GitHub repository and its protected workflow definition.

Run 'byeclaude licenses' to read the BSD license notices embedded for the Go terminal support packages. The release SBOM lists their pinned versions.

## Prebuilt GitHub Action

The Action has its own reviewed [engine pin](../action-release/README.md). SHA-256 values are stored in the Action source, and verified before the downloaded binary executes. It uses neither the user installer nor a mutable downloaded checksum as the trust anchor. A missing release, failed download or digest mismatch fails the guard; there is no Go build fallback.
