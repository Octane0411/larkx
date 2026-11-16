# Mermaid / PlantUML 路径

适用于：思维导图、时序图、类图、饼图、甘特图等结构化图表。

## 写入方式

1. 直接以 Mermaid / PlantUML 原生格式写入画板：
   ```bash
   lark-cli whiteboard +update --whiteboard-token <board_token> \
     --source @diagram.mmd --input_format mermaid --idempotent-token <unique_id> --as user
   ```
   （先加 `--dry-run` 校验参数；若画板已有内容且需整板替换，先确认后再传 `--overwrite`。）

2. 或借助 `@larksuite/whiteboard-cli` 本地预览、再转为 OpenAPI JSON 写入：
   ```bash
   npx -y @larksuite/whiteboard-cli@^0.2.13 -i diagram.mmd --to openapi --format json \
     | lark-cli whiteboard +update --whiteboard-token <board_token> \
         --source - --input_format raw --idempotent-token <unique_id> --as user
   ```
