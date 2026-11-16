# whiteboard +export（导出画板）

> **前置条件：** 先阅读 [`../../lark-shared/SKILL.md`](../../lark-shared/SKILL.md) 了解认证、全局参数和安全规则。

导出画板内容，支持导出为预览图片、SVG 矢量图、提取 PlantUML/Mermaid 代码，或获取飞书 OpenAPI 原生画板节点格式。

## 参数

| 参数                   | 必填 | 说明                                                                     |
|----------------------|----|------------------------------------------------------------------------|
| `--whiteboard-token` | 是  | 画板 token，需要拥有画板的读权限                                                    |
| `--output-type`      | 是  | 输出格式：`preview`（预览图片）、`svg`（SVG 矢量图）、`source`（PlantUML/Mermaid 代码）、`raw`（OpenAPI 原生画板节点格式） |
| `--output`           | 否  | 输出路径。当 `--output-type preview` 时必填；当 `--output-type svg/source/raw` 时可选，不填则直接输出到终端 |
| `--overwrite`        | 否  | 覆盖已存在的文件，默认为 false                                                     |

## 输出格式

- `preview`：预览图片。保存时会根据接口实际返回的 `Content-Type` 决定扩展名，例如 `image/jpeg` 会保存为 `.jpg`。
- `svg`：导出画板为标准 SVG 矢量图。可用于 SVG 编辑后回写画板（见 [`routes/svg-edit.md`](../routes/svg-edit.md)）。注意：导出为纯视觉快照，思维导图层级、表格结构、连接器绑定等语义信息会丢失。
- `source`：PlantUML/Mermaid 代码。仅限画板内有且仅有一个 PlantUML/Mermaid 图时，才可导出代码，否则会在返回值中告知不存在/有多个节点。
- `raw`：飞书 OpenAPI 原生画板节点 JSON 格式。如需构造复杂图形 DSL 并转换为 OpenAPI JSON，可参考 [`routes/dsl.md`](../routes/dsl.md)。
  - **需编辑后回写时，导出务必加 `--output <file>` 写入文件**：文件内容可直接作为 `+update` 的输入；直接输出到终端的结果会多一层 `{ ok, identity, data }` 包装，`+update` 无法解析。

## 示例

### 示例 1：导出画板为预览图片

```bash
lark-cli whiteboard +export \
  --whiteboard-token "wbcnxxxxxxxx" \
  --output-type preview \
  --output ./preview
```

### 示例 2：提取画板中的代码并直接输出

```bash
lark-cli whiteboard +export \
  --whiteboard-token "wbcnxxxxxxxx" \
  --output-type source
```

### 示例 3：导出画板为 SVG 矢量图

```bash
lark-cli whiteboard +export \
  --whiteboard-token "wbcnxxxxxxxx" \
  --output-type svg \
  --output ./whiteboard.svg \
  --as user
```

### 示例 4：导出画板原始节点结构到文件

```bash
lark-cli whiteboard +export \
  --whiteboard-token "wbcnxxxxxxxx" \
  --output-type raw \
  --output ./nodes.json \
  --overwrite
```
