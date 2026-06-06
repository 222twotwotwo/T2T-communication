# rag-go

`rag-go` 是对 LLMentor 中 Java `rag` 模块核心检索链路的 Go 版本重构。

该服务默认监听 `8001` 端口，尽量保持与 Java 版相同的 HTTP 路径和入参方式，方便作为独立微服务替换原 Spring Boot `rag` 模块。

## 已实现能力

- 文档读取：`txt`、`md`、`html`、`json`、`pdf`、`docx`
- 文档清洗：空白字符规整、异常字符过滤、重复段落去重
- 文档切分：重叠切分、递归切分、句子切分、Markdown 标题切分
- 向量化：直连 DashScope `text-embedding-v4`
- 向量存储：兼容 Java 版 `PgVectorStore` 的 `vector_st` 表
- 关键词检索：Elasticsearch `rag_docs` 索引，保留 IK 分词 mapping
- 混合检索：pgvector + Elasticsearch + RRF，支持 DashScope rerank
- 问答生成：直连 DashScope OpenAI-compatible Chat Completions
- 文件服务：MinIO URL 上传和 7 天预签名下载地址生成

## 未迁移范围

当前版本聚焦核心 RAG 检索链路，暂未迁移以下 Java 示例能力：

- GraphRAG / Neo4j
- Text2SQL
- 多模态 PDF 坐标级图文混排
- 高级 metadata filter
- Spring AI Modular Advisor 编排

## 启动方式

```powershell
cd rag-go
go mod tidy
$env:DASHSCOPE_API_KEY="你的 DashScope Key"
go run ./cmd/server
```

启动后访问：

```text
http://localhost:8001
```

## 配置

主配置文件是 `config.yaml`。常用配置项包括：

- `server.port`：服务端口，默认 `8001`
- `dashscope.api_key`：DashScope API Key，建议通过环境变量传入
- `postgres.dsn`：pgvector PostgreSQL 连接串
- `postgres.table`：向量表名，默认 `vector_st`
- `elasticsearch.url`：Elasticsearch 地址
- `elasticsearch.index`：ES 索引名，默认 `rag_docs`
- `minio.endpoint`：MinIO 地址
- `minio.bucket_name`：MinIO bucket
- `reader.tika_url`：可选 Tika 服务地址，用于增强 `doc` / `docx` / PDF 解析

支持以下环境变量覆盖配置：

```powershell
$env:RAG_GO_CONFIG="config.yaml"
$env:RAG_GO_PORT="8001"
$env:DASHSCOPE_API_KEY="sk-..."
$env:PG_DSN="postgres://pgvector:pgvector@localhost:5433/rag_test?sslmode=disable"
$env:ES_URL="http://localhost:9200"
$env:MINIO_ENDPOINT="http://localhost:9000"
$env:MINIO_ACCESS_KEY="minioadmin"
$env:MINIO_SECRET_KEY="minioadmin"
$env:MINIO_BUCKET="rag-test"
```

## 兼容接口

```text
GET/POST /rag/read?filePath=
GET/POST /rag/chunker?filePath=
GET/POST /rag/split?filePath=
GET/POST /rag/splitRecursive?filePath=
GET/POST /rag/splitSentence
GET/POST /rag/splitParent

GET/POST /rag/embedding/test
GET/POST /rag/embedding/embed?filePath=

GET/POST /rag/es/write?filePath=
GET/POST /rag/es/search?keyword=

GET/POST /rag/retriever/query?query=&threshold=0.5
GET/POST /rag/retriever/retrieve?query=&threshold=0.5
GET/POST /rag/retriever/retrieveAdvisor?query=

GET/POST /rag/hybrid/write?filePath=
GET/POST /rag/hybrid/searchFromEs?keyword=
GET/POST /rag/hybrid/searchFromVector?keyword=
GET/POST /rag/hybrid/searchFromHybrid?keyword=
GET/POST /rag/hybrid/chatToHybrid?keyword=

GET/POST /rag/files/upload?fileUrl=
GET/POST /rag/files/download-url/{objectName}
```

## 验证命令

```powershell
go test ./...
go build ./cmd/server
```

## 注意事项

- pgvector 表默认使用 `vector_st`，维度为 `768`，用于兼容 Java 版配置。
- Go 版不会在源码中硬编码 API Key，生产环境请使用环境变量或外部配置注入。
- Elasticsearch 需要提前安装 IK 分词器，否则 `rag_docs` 索引 mapping 创建会失败。
- 如果 `postgres.initialize_schema` 设置为 `true`，启动时会尝试创建 `vector` 扩展、向量表和 HNSW 索引。
