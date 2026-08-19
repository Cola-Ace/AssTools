# v0.3.0

## Highlights

- Add `--json` output to every command for stable, machine-readable results.
- Add normalized JSON summaries for `info`, including file metadata, structure, styles, events, and diagnostics.
- Add normalized JSON diagnostics and summary output for `check`, including its compliance status and exit behavior.
- Add normalized JSON previews and command metadata for `normalize` and `help`.
- Keep JSON output consistent with the existing command exit codes, including structured errors.

## Compatibility

- Existing command-line workflows remain supported.
- Human-readable output remains the default when `--json` is not specified.
- Release artifacts continue to target Windows, Linux, and macOS on `amd64` and `arm64`.
