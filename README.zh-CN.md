# asst

`asst` 是一个跨平台 ASS 字幕检查与规范化命令行工具。项目只使用 Go 标准库，并尽量保留未修改内容的原始字节、UTF-8 BOM、换行符和未知章节。

## 命令

```text
asst info [--strict] [-|--input|<input.ass>]
asst check [--ignore-vsfiltermod] [-|--input|<input.ass>]
asst normalize [--backup] [--output <path>] [--yes] [--matrix <auto|value>] <input.ass>
asst help [command]
```

`info` 汇总文件、章节、样式、字体、事件、时间和合规状态；传入 `-` 或 `--input` 可从标准输入读取。成功加载后，`info` 默认即使发现合规问题也返回 `0`；传入 `--strict` 时，发现规范错误或未解决的 manual 项返回 `1`。`check` 按 `path:line: severity[code]: message` 输出稳定诊断，也支持 `-`/`--input`。检查以 libass 标签语义为准，VSFilterMod 扩展标签会单独给出 warning；`--ignore-vsfiltermod` 只隐藏这些兼容性 warning，不会隐藏其语法错误。`normalize` 先预览安全修改，确认后替换并复查结果；传入 `--output <path>` 可将规范化结果写入另一文件，传入 `--yes` 可跳过确认提示。默认不会创建备份文件；传入 `--backup` 才会先写入与原文件逐字节一致的 `<input.ass>.bak`。

`auto` 会保留合法的现有 Matrix；缺失或非法时优先根据 `LayoutRes` 推断，1080p 为 `TV.709`、720p 为 `TV.601`，再回退到 `PlayRes`。显式值支持 `None`、`TV.601`、`TV.709`、`TV.240M`、`TV.FCC` 及对应的 `PC.*` 值，大小写不敏感并在预览/写回时使用规范大小写。

退出码：`0` = 成功、仅有 warning、用户取消，或非 strict 的 `info` 合规发现；`1` = 检查发现规范错误、strict `info` 合规发现，或规范化后仍有未解决的 manual 项；`2` = 参数、编码、I/O、备份或替换失败。

## 安装

从 Release 下载对应 Windows、Linux 或 macOS 的 `amd64`/`arm64` 二进制文件，直接将 `asst`/`asst.exe` 放入 `PATH`。

## 构建

```text
go build ./cmd/asst
go test ./...
```

发布脚本使用 `CGO_ENABLED=0`、`-trimpath` 和 `-ldflags="-s -w"` 构建 Windows、Linux、macOS 的 amd64/arm64 版本。

## 发布流程

每个合并到 `main` 的 PR 都会发布一个版本。请在 PR 中将 `VERSION` 更新为下一个 `vMAJOR.MINOR.PATCH`，并在 `RELEASE_NOTES.md` 中填写该版本面向用户的说明。Release 工作流会在 PR 每次更新时校验并构建预览产物，PR 合并后再创建对应的 tag 和 GitHub Release。
