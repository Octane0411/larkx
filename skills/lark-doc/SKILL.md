---
name: lark-doc
description: "飞书云文档（Docx / Wiki）内容操作：通过标准 Markdown 同步文档（docs push / docs pull），或读写结构化内容、管理图片附件与画板。用户提供文档 URL/token（包括 /docx/、/wiki/）或需要将本地 Markdown 与飞书文档同步时使用。表格或 Base 内部数据操作不在本 skill。"
metadata:
  requires:
    bins: ["lark-cli"]
    skills: ["lark-shared"]
  cliHelp: "lark-cli docs --help;lark-cli mindnotes --help"
---

# docs

## 核心原则

- **飞书文档只作为内容同步与读写载体**：保持用户或本地 Markdown 草稿的原始文风、措辞与结构，不套用任何内置写作模板或体裁规范。
- **身份**：文档操作默认优先使用当前登录用户身份（`--as user`）。

## 命令选择

### 1. 标准 Markdown 同步（默认首选）

- **本地 Markdown 新建或覆盖同步到飞书 — `docs push`（`+push`）**：
  - `larkx docs push <file.md>`：新建飞书文档（自动提取 `# 一级标题` 或 frontmatter `title` 作为文档标题，上传本地相对/绝对/`file://` 路径图片，自动转义正文裸 `<`）。
  - `larkx docs push <file.md> --doc "<Docx/Wiki URL 或 token>"`：覆盖更新已有文档（`--mode append` 可改为文末追加）。
- **飞书文档拉取为干净标准 Markdown — `docs pull`（`+pull`）**：
  - `larkx docs pull "<Docx/Wiki URL 或 token>" -o <file.md> --download-media`：导出标准 Markdown（自动还原 `# Title`、去掉多余反斜杠转义，并下载图片到本地目录）。

### 2. 读取与局部编辑

- **读取 / 摘要 / 评论上下文 — [`+fetch`](references/lark-doc-fetch.md)**：支持按 `outline`、`section`、`range`、`keyword` 局部读取。
- **创建（原生 XML / Markdown）— [`+create`](references/lark-doc-create.md)**：直接用 `--content` 传入 XML 或 Markdown 创建文档。
- **局部精确编辑 / block 直达链接 — [`+update`](references/lark-doc-update.md)**：文本替换（`str_replace`）或块级增删改移（`block_replace`、`block_insert_after`、`block_delete`）。
- **历史版本 — [`+history-list` / `+history-revert` / `+history-revert-status`](references/lark-doc-history.md)**：查询与回滚文档历史版本。

### 3. 资源、画板与思维笔记

- **插入本地素材 — [`+media-insert`](references/lark-doc-media-insert.md)**：在文末插入本地图片或附件。
- **预览素材 — [`+media-preview`](references/lark-doc-media-preview.md)**：预览文档或评论中的图片与附件。
- **下载素材 — [`+media-download`](references/lark-doc-media-download.md)**：下载文档中的图片、附件或画板缩略图。
- **Docx 封面 — [`+resource-download` / `+resource-update` / `+resource-delete`](references/lark-doc-resource-cover.md)**：管理文档封面。
- **画板 — [`画板语法参考`](references/lark-doc-whiteboard.md)**：在文档中插入 Mermaid / PlantUML / SVG 画板；编辑已有画板使用 [`lark-whiteboard`](../lark-whiteboard/SKILL.md)。
- **思维笔记 — `mindnotes`**：已有思维笔记参考 [`lark-doc-mindnote.md`](references/lark-doc-mindnote.md)。
- **XML 与扩展块语法（仅需 Callout / Grid / 扩展块时查阅）**：[`lark-doc-xml.md`](references/lark-doc-xml.md)、[`lark-doc-xml-extended-blocks.md`](references/lark-doc-xml-extended-blocks.md)、[`lark-doc-md.md`](references/lark-doc-md.md)。

### 认证与 Scope

仅在遇到未认证、token 或 scope 错误时查阅 [`lark-shared`](../lark-shared/SKILL.md)。

## 不在本 Skill 范围

- **Drive 文件级操作**（搜索、复制、移动、权限、独立评论）→ [`lark-drive`](../lark-drive/SKILL.md)。
- **Wiki 空间与节点层级管理** → [`lark-wiki`](../lark-wiki/SKILL.md)。
