---
name: lark-slides
version: 1.0.0
description: "飞书幻灯片：创建和编辑幻灯片。创建演示文稿、读取幻灯片内容、管理幻灯片页面（创建、删除、读取、局部替换）。当用户需要创建或编辑幻灯片、读取或修改单个页面时使用。当用户给出 doubao.com 的 /slides/ URL/token 时，也应直接使用本 skill，不要因为域名不是飞书而回退到 WebFetch；路由依据是 URL 路径模式和 token，而不是域名。不负责：云文档内容编辑（走 lark-doc）、云文档里的独立画板对象（走 lark-whiteboard）、上传或下载普通文件（走 lark-drive）。"
metadata:
  requires:
    bins: ["lark-cli"]
  cliHelp: "lark-cli slides --help"
---

# slides (v1)

## XML 语法与硬约束（常见服务端报错点）

- 画布尺寸为 `960x540`，元素坐标与尺寸应保持在页面边界内。
- `<slide>` 直接子元素仅支持 `<style>`、`<data>`、`<note>`；文本、图形、表格、图表放在 `<data>` 内。
- 文本通过 `<content><p>...</p></content>` 表达；字号显式设置 `<content>` 的 `fontSize` 属性，文字较多时设置 `wrap="true" autoFit="normal-auto-fit"` 避免溢出。
- 文字颜色使用 `<content>` 的 `color` 属性（非 `fontColor`）；行距使用 `lineSpacing="multiple:xx"` 或 `lineSpacing="fixed:xx"`。
- 图片使用 `<img src="...">`（非 `<image>`）。
- 原生图表（柱状/条形/折线/面积/饼环/雷达/组合）使用 `<chart>`；隐藏图例直接省略 `<chartLegend>`（不支持 `position="none"`）。示例见 [`xml/slides_chart_demo.xml`](references/xml/slides_chart_demo.xml)。
- `<table>` 必须显式设置 `width` 和 `height` 固定总尺寸，并设置 `<col width="...">` 与 `<tr height="...">`；`<td>` 直接子元素仅支持 `<fill>`、`<content>` 和边框配置，不能嵌套 `<shape>`、`<img>`、`<icon>`。
- `<shape type="rect">` 不是容器，其它元素需与之平级靠坐标叠放；渐变色使用 `linear-gradient(135deg, rgba(R,G,B,A) 0%, rgba(R,G,B,A) 100%)`（`rgba()` + 百分比停靠点）。
- IconPark 图标检索见 [`xml/iconpark.md`](references/xml/iconpark.md) 与 [`scripts/iconpark_tool.py`](scripts/iconpark_tool.py)，`<fill><fillColor color="rgba(R,G,B,A)"/></fill>` 控制填充色。

## Quick Reference

| 用户需求 | 动作与关键文档 |
|----------|----------------|
| 新建 PPT | 一步创建或两步逐页添加：[`cli/lark-slides-create.md`](references/cli/lark-slides-create.md)、[`cli/lark-slides-add-slide.md`](references/cli/lark-slides-add-slide.md) |
| 导入并编辑已有 PPTX | 先用 `drive +import --file <x.pptx> --type slides` 导入，再通过 `slides +xml-get`、`+replace-slide` / `+update-slide` 编辑 |
| 局部替换 / 插入块元素 | `slides +replace-slide`（[`cli/lark-slides-replace-slide.md`](references/cli/lark-slides-replace-slide.md)） |
| 整页覆盖写回 / 改背景 / 删减元素 | `slides +update-slide`（[`cli/lark-slides-update-slide.md`](references/cli/lark-slides-update-slide.md)；未写入 `--content` 的元素会被移除） |
| 追加 / 插入单页 | `slides +add-slide`（[`cli/lark-slides-add-slide.md`](references/cli/lark-slides-add-slide.md)，`--slide @file`） |
| 删除页面 | `slides +delete-slide`（[`cli/lark-slides-delete-slide.md`](references/cli/lark-slides-delete-slide.md)） |
| 读取全文或单页 XML | `slides +xml-get`（[`cli/lark-slides-xml-presentations-get.md`](references/cli/lark-slides-xml-presentations-get.md)） |
| 查看或回滚历史版本 | `+history-list` / `+history-revert`（[`cli/lark-slides-history.md`](references/cli/lark-slides-history.md)；回滚仅接受 `history_version_id`） |
| 页面截图 | `slides +screenshot`（[`cli/lark-slides-screenshot.md`](references/cli/lark-slides-screenshot.md)） |
| 上传 / 下载图片 | `slides +media-upload`（[`cli/lark-slides-media-upload.md`](references/cli/lark-slides-media-upload.md)）或 XML `<img src="@./path">` |
| XML 结构与 Schema 校验 | [`xml/xml-schema-quick-ref.md`](references/xml/xml-schema-quick-ref.md)、[`xml/slides_xml_schema_definition.xml`](references/xml/slides_xml_schema_definition.xml)、[`scripts/xml_lint.py`](scripts/xml_lint.py) |

**CRITICAL — 开始前 MUST 先读取 [`../lark-shared/SKILL.md`](../lark-shared/SKILL.md)，认证、权限和全局参数均以 lark-shared 为准。**

**CRITICAL — 查看或回滚历史版本前，MUST 先读取 [`cli/lark-slides-history.md`](references/cli/lark-slides-history.md)。回滚接口只接受 `history_version_id`，不要把 `revision_id` 直接传给 `+history-revert`。**

**CRITICAL — 生成 `<slide>` XML 之前，应先读取 [`xml/xml-schema-quick-ref.md`](references/xml/xml-schema-quick-ref.md) 核对标签和属性。**

## 身份选择

飞书幻灯片通常是用户自己的内容资源。**默认应优先显式使用 `--as user`（用户身份）执行 slides 相关操作**，始终显式指定身份。

- **`--as user`（推荐）**：以当前登录用户身份创建、读取、管理演示文稿。执行前先完成用户授权：

```bash
lark-cli auth login --domain slides
```

- **`--as bot`**：仅在用户明确要求以应用身份操作，或需要让 bot 持有/创建资源时使用。使用 bot 身份时，要额外确认 bot 是否真的有目标演示文稿的访问权限。

**执行规则**：

1. 创建、读取、增删 slide、按用户给出的链接继续编辑已有 PPT，默认都先用 `--as user`。
2. 如果出现权限不足，先检查当前是否误用了 bot 身份；不要默认回退到 bot。
3. 只有在用户明确要求"用应用身份 / bot 身份操作"，或当前工作流就是 bot 创建资源后再做协作授权时，才切换到 `--as bot`。

## 核心概念

### URL 格式与 Token

| URL 格式 | 示例 | Token 类型 | 处理方式 |
|----------|------|-----------|----------|
| `/slides/` | `https://example.larkoffice.com/slides/xxxxxxxxxxxxx` | `xml_presentation_id` | URL 路径中的 token 直接作为 `xml_presentation_id` 使用 |
| `/wiki/` | `https://xxx.feishu.cn/wiki/wikcn_EXAMPLE_NODE_TOKEN_123456` | `wiki_token` | ⚠️ **不能直接使用**，需要先查询获取真实的 `obj_token` |

> 带 `--presentation` 的 slides shortcut 会自动解析以上两种 URL。

### Wiki 链接特殊处理

知识库链接（`/wiki/TOKEN`）不能直接当 `xml_presentation_id`。使用 Slides shortcut 时直接传入链接，CLI 会查询节点、校验 `data.obj_type == "slides"` 并使用 `data.obj_token`。手动解析命令：

```bash
lark-cli wiki +node-get --node-token 'https://xxx.feishu.cn/wiki/wikcn_EXAMPLE_NODE_TOKEN_123456' --as user --format json
```

节点解析必须与后续 Slides 操作使用相同身份。

### 资源关系

```text
Wiki Space (知识空间)
└── Wiki Node (知识库节点, obj_type: slides)
    └── obj_token → xml_presentation_id

Slides (演示文稿)
├── xml_presentation_id (演示文稿唯一标识)
├── revision_id (版本号)
└── Slide (幻灯片页面)
    └── slide_id (页面唯一标识)
```

## Shortcuts

Slides 相关操作使用 shortcut（`lark-cli slides +<verb> [flags]`）：

| Shortcut | 说明 |
|----------|------|
| [`+create`](references/cli/lark-slides-create.md) | 创建 PPT，可选一步添加页面 |
| [`+add-slide`](references/cli/lark-slides-add-slide.md) | 向已有演示文稿追加或插入**一页**（`--before-slide-id` 控制位置），XML 支持 `@file` / stdin，`<img src="@./path">` 占位符自动上传 |
| [`+delete-slide`](references/cli/lark-slides-delete-slide.md) | 按 `slide_id` 删除**一页** |
| [`+xml-get`](references/cli/lark-slides-xml-presentations-get.md) | 读取全文或单页 XML；用 `--presentation` 指定演示文稿，单页传 `--slide-id` 或 `--slide-number` |
| [`+screenshot`](references/cli/lark-slides-screenshot.md) | 把幻灯片页面截图保存为本地图片；用 `--slide-number` 或 `--slide-id` 指定页面 |
| [`+media-upload`](references/cli/lark-slides-media-upload.md) | 上传本地图片到指定演示文稿，返回 `file_token`（用作 `<img src="...">`），最大 20 MB |
| `+media-download` | 根据 Slides 图片 `file_token` 下载本地图片 |
| [`+replace-slide`](references/cli/lark-slides-replace-slide.md) | 对已有幻灯片页面进行块级替换/插入（`block_replace` / `block_insert`），自动注入 id 和 `<content/>`，不改变页序 |
| [`+update-slide`](references/cli/lark-slides-update-slide.md) | 整页覆盖写回已有页面（`slide_id` 和页序不变，**未写进 `--content` 的元素会被删除**） |

## 核心规则

1. **创建流程**：新建演示文稿用 `slides +create`，一步创建还是两步创建按 [`cli/lark-slides-create.md`](references/cli/lark-slides-create.md) 判断
2. **`<slide>` 直接子元素只有 `<style>`、`<data>`、`<note>`**：文本和图形必须放在 `<data>` 内
3. **文本通过 `<content>` 表达**：必须用 `<content><p>...</p></content>`，不能把文字直接写在 shape 内；编写 `--parts` 时，`block_replace` 使用字段 `replacement`，`block_insert` 使用 `insertion`
4. **保存关键 ID**：后续操作需要 `xml_presentation_id`、`slide_id`、`revision_id`
5. **删除谨慎**：删除不可逆，删前先回读确认 `slide_id`
6. **编辑已有页面优先原链接更新**：修改单个 shape/img 用 `+replace-slide`，整页改动或改背景用 `+update-slide`，追加/插入单页用 `+add-slide`，删除单页用 `+delete-slide`
7. **`<img src>` 只能用上传到飞书 drive 的 `file_token`，禁止使用 http(s) 外链 URL**：先「本地图片 → `slides +media-upload` 上传或在 XML 里写 `<img src="@./path">` 占位符自动上传 → 写 `file_token`」，**图片最大 20 MB**（媒体上传不支持分片）。

> **注意**：如果 md 内容与 `xml/slides_xml_schema_definition.xml` 不一致，以后者为准。
