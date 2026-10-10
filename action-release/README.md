# Pinned Action engine

The composite Action downloads **v0.1.0-alpha.9** from the canonical release.
Its source is `98370d566d8e70a0f6e150614c438b88474c5801`.
The six binary hashes in this directory were verified against that release's
checksums, build provenance and GitHub artifact attestations. The release gate
also built the complete release set byte-for-byte twice.
[Build evidence](https://github.com/angusu-de/ByeClaude/actions/runs/38058927021).

Action revisions and CLI releases are versioned separately: the Action pins an
already published, reviewed engine. It never resolves `latest`, runs Go, reads
a downloaded checksum as its trust anchor, or falls back to an unchecked binary.
Pin the consumer's `uses` to a full Action commit SHA to also pin this manifest.

To update the engine, first publish and verify the complete new release set,
then change `version` and all six digest entries in a reviewed Action change.
Run the action-wrapper tests and the real Action on Windows, Linux and macOS.
Never derive these hashes during the consumer's workflow run.
