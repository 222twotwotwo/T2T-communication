# RAG 知识库集成说明

本项目采用“复用并小幅增强 rag-go”的方式接入知识库：T2T 不内置向量库，只在每轮对话前调用 rag-go 的检索接口，拿到同场景的知识片段后注入当前对话 LLM 的 prompt。rag-go 不直接生成对话答案，T2T 仍保留自己的角色扮演、发音评分、纠错和报告流程。

## 接入点

- T2T 检索触发点：`backend/internal/session/service.go` 的 `AddTurn`。
- T2T prompt 注入点：`backend/internal/providers/commercial.go` 的 `buildConversationPrompt`。
- rag-go 写入端点：`POST /rag/hybrid/write?filePath=<path>&category=<scenario-id>`。
- rag-go 检索端点：`POST /rag/hybrid/searchFromHybrid?keyword=<text>&category=<scenario-id>&topK=5&rerank=false`。

## 配置

`config/app.mock.json` 默认关闭 RAG，保持零密钥、零外部依赖启动。

正式启用时在配置中设置：

```json
"rag": {
  "enabled": true,
  "baseURL": "http://localhost:8001",
  "topK": 5,
  "timeoutMs": 1500,
  "useRerank": false
}
```

也可以用环境变量覆盖：

```powershell
$env:T2T_RAG_ENABLED="true"
$env:T2T_RAG_BASE_URL="http://localhost:8001"
$env:T2T_RAG_TOP_K="5"
$env:T2T_RAG_TIMEOUT_MS="1500"
$env:T2T_RAG_USE_RERANK="false"
```

## 导入内置示例知识

先启动 rag-go，并确保 DashScope、PostgreSQL/pgvector、Elasticsearch 已按 rag-go 的配置可用。然后在 T2T 根目录执行：

```powershell
.\scripts\ingest-rag.ps1 -RagBaseUrl "http://localhost:8001"
```

脚本会读取 `knowledge/scenarios/*.md`，用文件名作为 category，例如 `interview.md` 会写入 `category=interview`。T2T 会用当前场景 ID 作为检索 category，因此不同场景的知识不会互相串用。

## 扩充知识

新增或修改 `knowledge/scenarios/<scenario-id>.md` 后重新执行导入脚本即可。现有场景 ID 包括：

- `interview`
- `restaurant`
- `meeting`
- `travel`
- `small-talk`

如果新增 T2T 场景，请同步新增同名知识文件，或在导入时手动指定对应 category。
