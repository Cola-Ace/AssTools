# asst

[简体中文](README.zh-CN.md)

`asst` is a small cross-platform command-line checker and normalizer for ASS subtitle files. It uses only the Go standard library and keeps untouched source bytes, UTF-8 BOMs, line endings, and unknown sections intact.

## Commands

```text
asst info <input.ass>
asst check [--ignore-vsfiltermod] <input.ass>
asst normalize [--backup] [--yes] [--matrix <auto|value>] <input.ass>
asst help [command]
```

`info` summarizes the file, sections, styles, fonts, events, timing, and compliance. `check` emits stable diagnostics in `path:line: severity[code]: message` form. Tag checking follows libass semantics; VSFilterMod extensions receive separate warnings. `--ignore-vsfiltermod` hides only those compatibility warnings, while syntax errors remain visible. `normalize` previews safe edits, asks for confirmation, then replaces the original with a rechecked candidate; pass `--yes` to skip the confirmation prompt. It does not create a backup by default; pass `--backup` to write a byte-identical `<input.ass>.bak` first.

The default matrix mode is `auto`. It retains a legal existing value and infers `TV.709` from 1080p or `TV.601` from 720p, preferring `LayoutRes` over `PlayRes`. An explicit value accepts `None`, `TV.601`, `TV.709`, `TV.240M`, `TV.FCC`, and corresponding `PC.*` values (case-insensitive) and is shown in the preview.

```text
$ asst check episode.ass
episode.ass:2: warning[script-info-comment]: semicolon comment is present in Script Info

Summary: 0 errors, 1 warnings, 0 manual items
Status: compliant with warnings

$ asst normalize episode.ass
== Normalize preview ==
...
Apply 1 change to "episode.ass"?
Confirm [y/N]
```

Exit codes: `0` = success, warnings, or cancellation; `1` = compliance errors or unresolved manual items after normalization; `2` = usage, encoding, I/O, backup, or replacement failures.

## Installation

Download the binary for Windows, Linux, or macOS (`amd64` or `arm64`) from a release and place `asst`/`asst.exe` on your `PATH`.

## Building

```text
go build ./cmd/asst
go test ./...
```

Release builds are made with `CGO_ENABLED=0`, `-trimpath`, and `-ldflags="-s -w"` for Windows, Linux, and macOS amd64/arm64 targets.

## Release process

Every pull request merged into `main` publishes a release. Update `VERSION` with the next `vMAJOR.MINOR.PATCH` and put the user-facing notes for that version in `RELEASE_NOTES.md`. The Release workflow validates and builds a preview on each pull request update, then creates the matching tag and GitHub Release after the pull request is merged.
