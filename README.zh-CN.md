# asst

`asst` 是一个跨平台 ASS 字幕检查与规范化命令行工具。项目只使用 Go 标准库，并尽量保留未修改内容的原始字节、UTF-8 BOM、换行符和未知章节。

## 命令

```text
asst info <input.ass>
asst check [--ignore-vsfiltermod] <input.ass>
asst normalize [--backup] [--yes] [--matrix <auto|value>] <input.ass>
asst help [command]
```

`info` 汇总文件、章节、样式、字体、事件、时间和合规状态；`check` 按 `path:line: severity[code]: message` 输出稳定诊断。检查以 libass 标签语义为准，VSFilterMod 扩展标签会单独给出 warning；`--ignore-vsfiltermod` 只隐藏这些兼容性 warning，不会隐藏其语法错误。`normalize` 先预览安全修改，确认后替换并复查结果；传入 `--yes` 可跳过确认提示。默认不会创建备份文件；传入 `--backup` 才会先写入与原文件逐字节一致的 `<input.ass>.bak`。

`auto` 会保留合法的现有 Matrix；缺失或非法时优先根据 `LayoutRes` 推断，1080p 为 `TV.709`、720p 为 `TV.601`，再回退到 `PlayRes`。显式值支持 `None`、`TV.601`、`TV.709`、`TV.240M`、`TV.FCC` 及对应的 `PC.*` 值，大小写不敏感并在预览/写回时使用规范大小写。

退出码：`0` = 成功、仅有 warning 或用户取消；`1` = 检查发现规范错误，或规范化后仍有未解决的 manual 项；`2` = 参数、编码、I/O、备份或替换失败。

## 安装

从 Release 下载对应 Windows、Linux 或 macOS 的 `amd64`/`arm64` 二进制文件，直接将 `asst`/`asst.exe` 放入 `PATH`。

## 构建

```text
go build ./cmd/asst
go test ./...
```

发布脚本使用 `CGO_ENABLED=0`、`-trimpath` 和 `-ldflags="-s -w"` 构建 Windows、Linux、macOS 的 amd64/arm64 版本。
