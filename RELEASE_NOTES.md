# v0.2.0

## Highlights

- Add standard-input support to `info` and `check` with `-`/`--input`.
- Add `info --strict` for compliance-aware exit status.
- Add `normalize --output` to write a normalized candidate to another path.
- Improve matrix validation and canonicalization, including automatic resolution-based inference.
- Make in-place normalization transactional and recheck the candidate before committing, restoring the original when validation fails.
- Improve output reliability, section scanning, and handling of current-directory input paths.

## Compatibility

- Existing command-line workflows remain supported.
- Release artifacts continue to target Windows, Linux, and macOS on `amd64` and `arm64`.
