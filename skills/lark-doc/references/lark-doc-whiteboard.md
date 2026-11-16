# lark-doc 画板语法参考

在飞书文档中插入画板时，可使用 `<whiteboard>` XML 标签（支持 `mermaid`、`plantuml`、`svg` 或 `blank`）；若需查询或编辑已有画板，使用 [`lark-whiteboard`](../../lark-whiteboard/SKILL.md)。

## 插入语法

### 1. Mermaid

```xml
<whiteboard type="mermaid">
flowchart LR
  A[Input] --> B[Output]
</whiteboard>
```

也支持从本地文件读取：`<whiteboard type="mermaid" path="@./diagram.mmd"></whiteboard>`。

### 2. SVG / PlantUML

```xml
<whiteboard type="svg" path="@./diagram.svg"></whiteboard>
<whiteboard type="plantuml" path="@./sequence.puml"></whiteboard>
```

- SVG 需为自包含 `<svg ...>...</svg>`（含 `viewBox`，不依赖外链资源）；文字建议使用 `<text>` 元素。
- 避免使用 `<pattern>`、`<clipPath>`、`<mask>` 与非阴影用途的 `<filter>`，以免画板解析降级。

### 3. 复制已有画板或新建空白画板

- 复制已有画板：`<whiteboard src="wb_token_example"></whiteboard>`
- 新建空白画板：`<whiteboard type="blank"></whiteboard>`（创建或更新返回 `data.document.new_blocks[].block_token`）

## 预览校验

```bash
larkx whiteboard +export --whiteboard-token "<board_token>" --output-type preview --output ./preview.png
```
