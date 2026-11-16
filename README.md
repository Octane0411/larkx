# larkx

[![CI](https://github.com/Octane0411/larkx/actions/workflows/ci.yml/badge.svg)](https://github.com/Octane0411/larkx/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/Octane0411/larkx)](https://github.com/Octane0411/larkx/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](./LICENSE)

> A cleaner, Markdown-first build of [larksuite/cli](https://github.com/larksuite/cli) (`lark-cli`).

`larkx` 基于官方开源仓库 [github.com/larksuite/cli](https://github.com/larksuite/cli)（MIT License，基于提交 [`3d82c85419ad7560ad017db7a3972a596b5fbeb1`](https://github.com/larksuite/cli/commit/3d82c85419ad7560ad017db7a3972a596b5fbeb1)）分支演进，保留全部 18+ 业务域能力，**零配置复用现有的 `~/.lark-cli` 配置与系统钥匙串登录态**，并解决原版 CLI 的两类痛点：

1. **剔除冗长流程与文体污染**：移除 `docs +script`（`init-draft` / 手写 XML / 删除草稿目录）以及 `lark-doc-create-workflow`、体裁契约、Slides「豆包设计原则」/ 规划层、Sheets 视觉规范、IM 卡片风格指南、Whiteboard 创作工作流等主观/模板化 prompt，仅保留客观 API 与语法说明，不干扰模型与作者自身文风。
2. **标准 Markdown 一键无损同步（`docs push` / `docs pull`）**：
   - 支持任意工作目录外的本地图片（相对源 `.md` 文件路径、`../`、绝对路径、`file://` URL），自动校验安全黑名单、解码尺寸、上传至云盘媒体库并绑定；
   - 自动从 YAML frontmatter `title` 或首个 `# H1` 提取 `<title>` 并去重（可通过 `--keep-h1` 保留正文标题）；
   - 自动保护 fenced/inline 代码块、`$...$` / `$$...$$` 公式与 `\$` 货币符号、表格 `<br/>` 与 `<https://...>` 自动链接，并将正文裸 `<tag>` 转义为 `\<tag>`，防止被服务端误解析吞字；
   - `docs pull` 自动清除 `docs_ai` 导出时的多余反斜杠转义（保留 `\\` 与 `\$`）、还原 `# 标题`，可选 `--download-media` 一键拉取图片（含 `feishu.cn/file/...` 等链接）并重写本地相对路径。

---

## 快速上手（同事安装指南）

### 方式一：一行命令预编译安装（推荐，无需 Go 环境）

支持 macOS（Apple Silicon / Intel）与 Linux（x86_64 / arm64），会自动下载 GitHub Release 预编译包并校验 SHA-256：

```bash
# 安装到 ~/.local/bin/larkx，并接管 `lark-cli`（推荐，Agent/脚本无感切换）
curl -fsSL https://raw.githubusercontent.com/Octane0411/larkx/main/scripts/install.sh | bash -s -- --takeover

# 或仅独立安装 `larkx`，不动现有 `lark-cli`
curl -fsSL https://raw.githubusercontent.com/Octane0411/larkx/main/scripts/install.sh | bash
```

### 方式二：从源码构建安装（需 Go >= 1.23）

```bash
git clone https://github.com/Octane0411/larkx.git
cd larkx
make takeover        # 构建安装到 ~/.local/bin/larkx 并启用 takeover
# 或仅安装 larkx：make install-local
```

### 登录态说明

- **本机已装过并登录过原版 `lark-cli`**：无需重新登录，直接执行 `larkx whoami`（以及 `lark-cli --version`）即可验证。
- **新机器首次使用**：运行 `larkx config init --new` 与 `larkx auth login --recommend`（详见 [`README.upstream.md`](./README.upstream.md)）。

### Shell `PATH` 优先级注意（macOS + Homebrew 必读）

安装器与 `takeover` 会在 `~/.local/bin/lark-cli` 创建指向 `~/.local/bin/larkx` 的软链（绝不修改 `/opt/homebrew/bin/lark-cli`）。请确认 `~/.local/bin` 在 `PATH` 中位于 `/opt/homebrew/bin` **之前**；尤其检查 `~/.zprofile` 与 `~/.bash_profile` 中是否先写了 `PATH="$HOME/.local/bin:$PATH"` 而**后**又执行了 `eval "$(/opt/homebrew/bin/brew shellenv)"`（如果顺序反了，`zsh -lc` 等非交互式登录 Shell 会重新把 `/opt/homebrew/bin` 放到最前，导致调用到旧版原生 CLI）。执行 `larkx takeover status` 可直接检查当前是否接管生效。

---

## 核心新增能力

### 1. `docs push`（或 `docs +push`）— 标准 Markdown 推送到飞书文档

```bash
# 位置参数直接创建新文档（自动提取 # H1 / frontmatter title，上传相对路径或 file:// 图片）
larkx docs push ./report.md

# 指定目标云盘文件夹或知识库链接
larkx docs push /abs/path/spec.md --parent-token "https://example.feishu.cn/drive/folder/<token>"

# 覆盖更新或追加到已有文档
larkx docs push ./spec.md --doc "https://example.feishu.cn/docx/<token>" --mode overwrite
larkx docs push ./changelog.md --doc "<token>" --mode append

# 从 stdin 读取并用 --dry-run 预览请求体
cat notes.md | larkx docs push - --title "Meeting Notes" --dry-run
```

常用参数：

| 参数 | 说明 |
| --- | --- |
| `<file.md>` / `--file` | 本地 `.md` 文件路径（或 `-` 读取 stdin） |
| `--content` | 内联 Markdown 字符串（替代 `--file`） |
| `--doc` | 目标文档 URL/token（省略则创建新文档） |
| `--mode` | 更新模式：`overwrite`（默认）\| `append` |
| `--title` | 显式指定文档标题（优先级高于 frontmatter 与 `# H1`） |
| `--keep-h1` | 在正文中同时保留首个 `# H1` |
| `--keep-frontmatter` | 在正文中保留 YAML frontmatter |
| `--allow-xml-tags` | 透传原始 DocxXML 标签而不转义正文裸 `<` |
| `--parent-token` | 新建文档的目标文件夹 / Wiki URL 或 token |

### 2. `docs pull`（或 `docs +pull`）— 拉取为干净本地 Markdown

```bash
# 输出纯 Markdown 到 stdout（方便管道重定向）
larkx docs pull "https://example.feishu.cn/docx/<token>" > spec.md

# 原子写入文件并下载文档内图片到 spec.assets/
larkx docs pull "https://example.feishu.cn/docx/<token>" -o ./spec.md --download-media

# 保留服务端原始导出格式不做反转义
larkx docs pull "<token>" --raw
```

### 3. `larkx takeover` — 可选软链托管 `lark-cli`

当 Codex / Claude Code 等 Agent 或历史脚本硬编码调用 `lark-cli` 时，可用内置命令在 `~/.local/bin/lark-cli` 建立指向 `larkx` 的软链：

```bash
larkx takeover status   # 查看当前软链与 PATH 优先级
larkx takeover enable   # 创建 ~/.local/bin/lark-cli -> ~/.local/bin/larkx
larkx takeover disable  # 仅移除指向 larkx 的托管软链
```

### 4. 默认安静与防覆盖保护

- 默认设置 `LARKSUITE_CLI_NO_UPDATE_NOTIFIER=1` 与 `LARKSUITE_CLI_NO_SKILLS_NOTIFIER=1`，去除输出中的 `_notice` 升级提示噪音（如需开启可设置 `LARKX_ENABLE_UPSTREAM_NOTIFIERS=1`）。
- `larkx update` 默认开启自保护（`LARKX_BINARY_GUARD=1`），拒绝被上游 `lark-cli` 发布包直接覆盖当前二进制；`larkx update --check` 仍可用，确需强制替换需显式传 `--upstream-force`。

---

## 开发、测试与构建

```bash
# 代码格式与静态检查
gofmt -l .
go vet ./...

# 运行脚本与内置 skill 格式检查（含 install.sh 端到端安装测试）
make script-test

# 运行 Go 单元测试（含 skills 纯净度校验、Markdown 同步与 takeover 单测）
go test -count=1 . ./cmd/... ./internal/... ./shortcuts/... ./extension/...

# 全量构建
make build
```

---

## 与上游保持同步

上游原版文档与完整说明保留在 [`README.upstream.md`](./README.upstream.md)。本仓库将上游配置为只读 `upstream` remote（禁用 push）：

```bash
git fetch upstream
git rebase upstream/main
```

## License

遵循上游项目一致的 [MIT License](./LICENSE)。
