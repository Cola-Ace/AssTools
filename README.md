# asst

[简体中文](README.zh-CN.md)

`asst` is a small cross-platform command-line checker and normalizer for ASS subtitle files. It uses only the Go standard library and keeps untouched source bytes, UTF-8 BOMs, line endings, and unknown sections intact.

## Commands

```text
asst info [--strict] [--json] [-|--input|<input.ass>]
asst check [--ignore-vsfiltermod] [--json] [-|--input|<input.ass>]
asst normalize [--backup] [--output <path>] [--yes] [--json] [--matrix <auto|value>] <input.ass>
asst help [--json] [command]
```

`info` summarizes the file, sections, styles, fonts, events, timing, and compliance. Pass `-` or `--input` to read its input from standard input. It returns `0` after a successful load even when compliance findings are present; pass `--strict` to return `1` for compliance errors or unresolved manual items. `check` emits stable diagnostics in `path:line: severity[code]: message` form and also accepts `-`/`--input`. Tag checking follows libass semantics; VSFilterMod extensions receive separate warnings. `--ignore-vsfiltermod` hides only those compatibility warnings, while syntax errors remain visible. `normalize` previews safe edits, asks for confirmation, then replaces the original with a rechecked candidate; pass `--output <path>` to write the candidate elsewhere, or `--yes` to skip the confirmation prompt. It does not create a backup by default; pass `--backup` to write a byte-identical `<input.ass>.bak` first.

The default matrix mode is `auto`. It retains a legal existing value and infers `TV.709` from 1080p or `TV.601` from 720p, preferring `LayoutRes` over `PlayRes`. An explicit value accepts `None`, `TV.601`, `TV.709`, `TV.240M`, `TV.FCC`, and corresponding `PC.*` values (case-insensitive) and is shown in the preview.

Pass `--json` to any command for one normalized JSON document on standard output. JSON output includes stable command/status fields, summaries, diagnostics, and command-specific metadata; `help --json` returns command metadata. `normalize --json` returns a preview without prompting; add `--yes` to apply the changes.

In `info --json`, `structure.matrix_candidate` is the canonical matrix value; inference context is reported separately in `structure.matrix_candidate_reason`.

Each `styles.definitions` entry is the complete style value map. When all style definitions share one format, that format is emitted once as `styles.fields`.

Style value keys use consistent snake_case names such as `font_name`, `font_size`, and `primary_colour`.

When all style definitions share one format, that format is emitted once as `styles.fields`; per-definition `fields` are only included when formats differ.

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

Exit codes: `0` = success, warnings, or cancellation (and non-strict `info` findings); `1` = compliance errors or unresolved manual items after normalization, or strict `info` findings; `2` = usage, encoding, I/O, backup, or replacement failures.

## Installation

Download the binary for Windows, Linux, or macOS (`amd64` or `arm64`) from a release and place `asst`/`asst.exe` on your `PATH`.

## Building

```text
go build ./cmd/asst
go test ./...
```

Release builds are made with `CGO_ENABLED=0`, `-trimpath`, and `-ldflags="-s -w"` for Windows, Linux, and macOS amd64/arm64 targets.

## Release process

Normal pull requests do not publish a release. When preparing a release, open a dedicated pull request that updates `VERSION` with the next `vMAJOR.MINOR.PATCH` and puts the user-facing notes for that version in `RELEASE_NOTES.md`. The Release workflow validates and builds a preview only for pull requests that change those files, then creates the matching tag and GitHub Release after the release pull request is merged.
