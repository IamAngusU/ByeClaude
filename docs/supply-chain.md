# Release integrity and reproducibility

## Installation

The installers default to the published `v0.1.0-alpha.1` tag. This is explicit
because GitHub's `latest` download endpoint excludes prereleases. Override
`BYECLAUDE_VERSION` to select another published tag; use `latest` only when a
stable release exists. The development terminal menu, saved blacklist and
new setup commands are not in alpha.1 yet; build the updated source until
a newer release passes CI.

On Windows, download and inspect `install.ps1`, then run:

```powershell
powershell -NoProfile -File .\install.ps1 -AddToPath
```

`-AddToPath` adds the install directory to your **user** PATH without admin
rights or changing system PATH. Reopen other cmd/PowerShell windows afterwards.
Omit it to manage PATH yourself. `BYECLAUDE_INSTALL_DIR` selects the install
directory. Existing binaries survive a failed checksum verification.

For development from a checkout, build `go build -trimpath -o
dist/byeclaude.exe ./cmd/byeclaude` on Windows (omit `.exe` on Unix). Keep that
binary at a persistent path before installing Git hooks.

## Release artifacts

ByeClaude release tags pass the same rewrite, fixture, race, security, installer
and attribution gates used for normal changes. A release publishes six static
binaries: Linux, macOS and Windows on amd64 and arm64.

The release set also contains:

- `SHA256SUMS.txt`, covering every binary, the SBOM and the provenance record;
- `SBOM.cdx.json`, a CycloneDX 1.5 inventory read from the linked Go build;
- `BUILD-PROVENANCE.json`, binding artifact hashes to the repository, exact
  source commit, source timestamp, release version and Go toolchain.

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

`BUILD-PROVENANCE.json` deliberately states `"signature_status": "unsigned"`.
Checksums detect corruption after a trusted download, and the SBOM exposes the
linked dependency inventory, but neither one authenticates a compromised
GitHub account. The project does not claim signed releases or SLSA provenance
until an independently verifiable signing identity and transparency-log flow
are actually configured.
