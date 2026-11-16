---
name: lark-doc
description: "飞书云文档（Docx / Wiki）内容操作：通过标准 Markdown 同步文档（docs push / docs pull），或读写结构化内容与管理附件。用户提供文档 URL/token（包括 /docx/、/wiki/）或需要将本地 Markdown 与飞书文档同步时使用。表格或 Base 内部数据操作不在本 skill。"
metadata:
  requires:
    bins: ["lark-cli"]
  cliHelp: "lark-cli docs --help;lark-cli mindnotes --help"
---

# docs

## 核心原则（写作与搬运解耦）

1. **先写纯标准 Markdown，再原封不动推送**：新建或整篇更新飞书文档时，先像写普通本地技术文档一样写好 `.md`，保持自然克制的文风与排版（**不要为了“适应飞书”额外加 `<callout>`、颜色等标签，不要把每个数字都加粗，也不要在动笔前阅读 `lark-doc-xml.md`、`lark-doc-update.md` 或 `lark-doc-whiteboard.md`**）。
2. **一条命令完成同步**：定稿后直接运行 `lark-cli docs push <file.md>`（或 `larkx docs push <file.md>`）；工具会自动提取首行 `# 标题` / YAML frontmatter `title`、上传任意相对/绝对/`file://` 本地图片并安全转义正文裸 `<`。命令返回 `ok: true` 即交付完成，**无需再跑多轮 `docs +fetch` + `docs +update` 微调排版**。
3. **身份**：文档操作默认优先使用当前登录用户身份（`--as user`）。

## 首选命令：标准 Markdown 同步

- **推送本地 Markdown 新建或更新飞书文档 — `docs push`（`+push`）**：
  - `lark-cli docs push <file.md>`：新建飞书文档（自动提取 `# 一级标题` 或 frontmatter `title`）。
  - `lark-cli docs push <file.md> --doc "<Docx/Wiki URL 或 token>"`：覆盖更新已有文档（`--mode append` 可改为文末追加）。
  - `lark-cli docs push <file.md> --title "..." --keep-h1`：自定义文档标题并同时保留正文一级标题。
- **拉取飞书文档为干净标准 Markdown — `docs pull`（`+pull`）**：
  - `lark-cli docs pull "<Docx/Wiki URL 或 token>" -o <file.md> --download-media`：导出标准 Markdown（自动还原 `# Title`、去除多余反斜杠转义，并下载文档内图片到本地目录）。

## 按需参考（仅在局部编辑已有文档或用户明确指定扩展块时查阅）

- **读取 / 搜索已有文档**：[`+fetch`](references/lark-doc-fetch.md)（支持 `outline`、`section`、`range`、`keyword`）。
- **局部改写已有文档**：[`+update`](references/lark-doc-update.md)（`str_replace`、`block_replace`、`block_insert_after`、`block_delete`）。
- **底层原生创建**：[`+create`](references/lark-doc-create.md)。
- **历史版本 / 媒体 / 封面 / 思维笔记**：[`+history-*`](references/lark-doc-history.md)、[`+media-insert`](references/lark-doc-media-insert.md)、[`+media-preview`](references/lark-doc-media-preview.md)、[`+media-download`](references/lark-doc-media-download.md)、[`+resource-*`](references/lark-doc-resource-cover.md)、[`lark-doc-mindnote.md`](references/lark-doc-mindnote.md)。
- **特殊块语法（仅用户明确要求 Callout / Grid / 画板 / 扩展块时查阅）**：[`lark-doc-xml.md`](references/lark-doc-xml.md)、[`lark-doc-xml-extended-blocks.md`](references/lark-doc-xml-extended-blocks.md)、[`lark-doc-whiteboard.md`](references/lark-doc-whiteboard.md)、[`lark-doc-md.md`](references/lark-doc-md.md)。
- **认证与权限报错排查**：仅在返回未登录或缺 scope 错误时查阅 [`lark-shared`](../lark-shared/SKILL.md)。

## 不在本 Skill 范围

- **Drive 文件级操作**（搜索、复制、移动、权限、独立评论）→ [`lark-drive`](../lark-drive/SKILL.md)。
- **Wiki 空间与节点层级管理** → [`lark-wiki`](../lark-wiki/SKILL.md)。
