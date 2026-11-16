# docs +create / docs push（创建飞书云文档）

## 推荐：标准 Markdown 一键同步（`docs push`）

如果本地已有标准 Markdown 文件（包含 `# 标题`、GFM 表格、公式或本地路径图片），优先使用 `docs push`：

```bash
# 从本地 Markdown 新建云文档
larkx docs push ./report.md

# 指定目录或知识库节点下新建
larkx docs push ./report.md --parent-token "<folder_or_wiki_url_or_token>"
```

## 原生 `docs +create`

需要直接使用 DocxXML（如 Callout、分栏 Grid、画板）或显式指定 `--content` 时使用 `docs +create`。写入前可按需查阅 [`lark-doc-xml.md`](lark-doc-xml.md) 或 [`lark-doc-md.md`](lark-doc-md.md)。

```bash
larkx docs +create --doc-format markdown --title "文档标题" --content "@./draft.md"
larkx docs +create --doc-format xml --content "@./draft.xml"
```

## 返回值

```json
{
  "ok": true,
  "identity": "user",
  "data": {
    "document": {
      "document_id": "DocTokenExample123",
      "revision_id": 1,
      "url": "https://example.feishu.cn/docx/DocTokenExample123",
      "new_blocks": [
        { "block_id": "blkExample", "block_type": "whiteboard", "block_token": "boardExample" }
      ]
    },
    "warnings": [],
    "tips": ""
  }
}
```

- **`document.new_blocks`**：本次操作新增的 block 列表。`block_id` 可用于 `docs +update`；`block_token` 是资源块 token。
- **`warnings`**：服务端返回的警告列表。
- **`permission_grant`**：仅以 bot 身份创建时返回；默认会自动为当前 CLI 用户授予 `full_access`。

## 参数

|参数|必填|说明|
|-|-|-|
|`--title`|否|文档标题|
|`--content`|视情况|文档内容（XML 或 Markdown 格式）；不传 `--content` 时必须传 `--title`|
|`--reference-map`|否|结构化 `reference_map` JSON object|
|`--doc-format`|否|`xml`（默认）\| `markdown`|
|`--parent-token`|否|父文件夹或知识库节点 token（与 `--parent-position` 互斥）|
|`--parent-position`|否|父节点位置，如 `my_library`（与 `--parent-token` 互斥）|

## 回查文档

用 `larkx docs +fetch --doc "<document_id 或文档 URL>" --detail with-ids` 回查，参见 [`+fetch`](lark-doc-fetch.md)。
