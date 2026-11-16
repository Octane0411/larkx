# DSL 路径

> **这是画板，不是网页。** 画板是无限画布上自由放置元素，flex 布局是可选增强。

## 步骤概览

```
Step 1: 读取元素语法参考
  - 读 elements/ 模块 — schema、layout、connectors、typography、image

Step 2: 生成完整 DSL JSON
  - 按 layout.md 选择布局模式和间距
  - 查看可用图标：`npx -y @larksuite/whiteboard-cli@^0.2.13 --icons`
  - 按 schema.md 语法输出完整 JSON，连线参考 connectors.md，排版参考 typography.md
  - 复杂生成逻辑建议保存为 `.cjs` 脚本执行产出 diagram.json

Step 3: 渲染验证 & 写入画板
  - 本地渲染预览图（仅用于校验）：`npx -y @larksuite/whiteboard-cli@^0.2.13 -i diagram.json -o diagram.png`
  - 转换为 OpenAPI 格式并 pipe 给 `+update`：
      npx -y @larksuite/whiteboard-cli@^0.2.13 -i diagram.json --to openapi --format json \
        | lark-cli whiteboard +update --whiteboard-token <board_token> \
            --source - --input_format raw --idempotent-token <时间戳+标识> --as user
```

**布局策略快速判断**（详见 `elements/layout.md`）：

先定**主布局**，再定子布局：**结构化信息**优先用 Flex，**关系链路**优先用 Dagre，**灵活定位**用绝对布局。

> **构建方式说明**：当使用脚本生成 JSON 时，建议使用 `.cjs`（CommonJS）并用 `node` 执行产出 JSON 文件。

## 模块索引

### 核心参考

| 模块     | 文件                         | 说明                            |
| -------- |----------------------------| ------------------------------- |
| DSL 语法 | `elements/schema.md`       | 节点类型、属性、尺寸值          |
| 信息密度 | `elements/content.md`    | 信息组织、分组与精简参考        |
| 布局系统 | `elements/layout.md`     | 网格方法论、Flex 映射、间距规则 |
| 排版规则 | `elements/typography.md` | 字号层级、对齐、行距            |
| 连线系统 | `elements/connectors.md` | 拓扑规划、锚点选择              |
| 图片元素 | `elements/image.md`      | 图片节点写法与约束              |

## 渲染前自查

- [ ] 不同分组用了不同颜色？同组节点样式完全一致？
- [ ] 外层浅色背景、内层白色节点？
- [ ] 所有节点有边框（borderWidth=2）？文字在背景上清晰可读？
- [ ] 连线用灰色（#BBBFC4），不用彩色？
- [ ] frame 都写了 layout 属性？gap 和 padding 都显式设置了？
- [ ] 含文字节点 height 用 fit-content？connector 在顶层 nodes 数组？

## 症状→修复表

| 看到的问题         | 改什么                              |
| ------------------ | ----------------------------------- |
| 文字被截断         | height 改为 fit-content             |
| 文字溢出容器右侧   | 增大 width，或缩短文字              |
| 节点重叠粘连       | 增大 gap                            |
| 节点挤成一团       | 增大 padding 和 gap                 |
| 连线穿过节点       | 调整 fromAnchor/toAnchor 或增大间距 |
| 大面积空白         | 缩小外层 frame 宽度                 |
| 文字和背景色太接近 | 调整 fillColor 或 textColor         |
| 布局整体偏左/偏右  | 调整绝对定位的 x 坐标使内容居中     |

## 关键约束速查

1. **含文字节点的 height 必须用 `'fit-content'`** — 写死数值会截断文字
2. **`fill-container` 仅在 flex 父容器中生效** — `layout: 'none'` 下宽度退化为 0
3. **`layout: 'none'` 的容器必须有固定宽高** — 不要写成 `fit-content`
4. **connector 必须放在顶层 nodes 数组** — 不能嵌套在 frame children 里
5. **flex 容器内的 x/y 会被完全忽略** — 需要自由定位时用 `layout: 'none'`
6. **Dagre 子容器默认为不透明节点** — 需穿透时声明 `layout: "dagre"` + `layoutOptions: { isCluster: true }`
