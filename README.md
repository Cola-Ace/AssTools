# asst

`asst` is a small cross-platform command-line checker and normalizer for ASS subtitle files. It uses only the Go standard library and keeps untouched source bytes, UTF-8 BOMs, line endings, and unknown sections intact.

## Commands

```text
asst info <input.ass>
asst check <input.ass>
asst normalize [--matrix <auto|value>] <input.ass>
asst help [command]
```

`info` summarizes the file, sections, styles, fonts, events, timing, and compliance. `check` emits stable diagnostics in `path:line: severity[code]: message` form. `normalize` previews safe edits, asks for confirmation, writes a byte-identical `<input.ass>.bak`, then replaces the original with a rechecked candidate.

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
Backup: "episode.ass.bak" [y/N]
```

Exit codes are `0` for success, warnings, or cancellation; `1` for compliance errors or unresolved manual items after normalization; and `2` for usage, encoding, I/O, backup, or replacement failures. The first release handles one valid UTF-8 `.ass` file at a time. JSON, batch directories, stdin, non-interactive `--yes`, SSA/ASS2, 32-bit targets, auto-update, signing, notarization, and package-manager distribution are intentionally out of scope.

## Installation

Download the binary for Windows, Linux, or macOS (`amd64` or `arm64`) from a release and place `asst`/`asst.exe` on your `PATH`. macOS binaries are unsigned and not notarized.

## Building

```text
go build ./cmd/asst
go test ./...
```

Release builds are made with `CGO_ENABLED=0`, `-trimpath`, and `-ldflags="-s -w"` for Windows, Linux, and macOS amd64/arm64 targets. macOS artifacts are unsigned and not notarized in this release.

## License

MIT; see [LICENSE](LICENSE).
