# T2T 英语口语练习工具

T2T 是一款面向真实场景的英语口语练习工具。当前版本已经包含 Go + Gin 后端、React + TypeScript + Vite 前端、可本地全链路运行的模拟适配器，以及可切换到商用供应商的适配器边界。

核心原则：

- 可落地部署：默认 mock 模式零密钥启动；commercial 模式通过配置文件切换供应商。
- 低延迟：会话层支持短轮次 REST 与实时 WebSocket 两种入口，便于后续接入流式 ASR/LLM/TTS。
- 纠错精准：会话中静默采集发音、语法、表达问题；结束后统一生成温和、可执行的课后报告。

## 技术选型

| 层级 | 选型 | 原因 |
| --- | --- | --- |
| 后端 | Go + Gin | 轻量、并发性能稳定、部署简单，适合低延迟语音编排服务。 |
| 前端 | React + TypeScript + Vite | 交互开发效率高，适合构建录音、会话、评分报告等复杂状态 UI。 |
| ASR | Provider 抽象，默认 mock | 方便本地演示；正式环境可切 Azure Speech 或其他流式 ASR。 |
| TTS | Provider 抽象，默认 mock，正式 edge-tts | mock 模式用浏览器 speechSynthesis 回退；正式环境用 edge-tts 生成自然语音。 |
| 发音评测 | Provider 抽象，默认 mock，正式 Azure Speech Pronunciation Assessment | Azure 发音测评在发音、流利度、完整度等维度成熟，适合量化反馈。 |
| LLM | Provider 抽象，支持 OpenAI / Anthropic Claude | OpenAI 用于低延迟对话与纠错，Claude 可用于高质量总结报告或备用模型。 |

## 项目结构

```text
.
├── backend
│   ├── cmd/server
│   └── internal
│       ├── api
│       ├── config
│       ├── domain
│       ├── providers
│       ├── scenarios
│       └── session
├── config
│   ├── app.mock.json
│   └── app.commercial.example.json
├── frontend
│   ├── src
│   └── vite.config.ts
└── docker-compose.yml
```

## 本地运行

后端：

```powershell
cd backend
go mod tidy
go run ./cmd/server
```

前端：

```powershell
cd frontend
npm install
npm run dev
```

默认访问：

- 前端：http://localhost:5173
- 后端：http://localhost:8080
- 健康检查：http://localhost:8080/api/health

## 配置

默认读取 `config/app.mock.json`，无需密钥即可跑通完整链路。

正式模式可以复制 `config/app.commercial.example.json` 后设置：

```powershell
$env:T2T_CONFIG_FILE="D:\community\project\T2T\config\app.commercial.json"
go run ./cmd/server
```

关键配置：

- `providers.mode`: `mock` 或 `commercial`
- `providers.asr`: `mock` / `azure`
- `providers.tts`: `mock` / `edge-tts`
- `providers.pronunciation`: `mock` / `azure`
- `providers.llm`: `mock` / `openai` / `anthropic`

## API 概览

- `GET /api/health`: 服务健康检查。
- `GET /api/scenarios`: 获取面试、点餐、会议等练习场景。
- `GET /api/provider-status`: 查看当前 provider 模式与配置状态。
- `POST /api/sessions`: 创建练习会话。
- `POST /api/sessions/:id/turn`: 提交一次用户发言，返回 AI 回复与量化信号。
- `POST /api/sessions/:id/finish`: 结束会话并生成课后总结。
- `GET /api/realtime/:id`: WebSocket 实时通道，支持文本与音频 chunk 事件。

## 纠错策略

1. 对话中只显示自然回复和轻量分数，不直接打断用户。
2. 发音问题、语法问题、表达问题进入会话级隐藏采集池。
3. 单次会话结束后统一输出纠错报告，包含问题、建议表达、原因和练习动作。
4. 评分维度包含 pronunciation、fluency、grammar、vocabulary、interaction，便于持续追踪。

## PR 规划与完成状态

### PR-01 项目骨架与运行基线

状态：已完成

交付：

- 创建 Go workspace、Gin 后端入口、Vite React 前端入口。
- 添加 mock 配置、commercial 示例配置、Docker Compose。
- 提供 README、运行命令和 API 概览。

验收：

- 后端可暴露 `/api/health`。
- 前端可通过 Vite 启动并代理 `/api`。
- mock 模式不需要任何密钥。

### PR-02 场景建模与会话生命周期

状态：已完成

交付：

- 支持面试、点餐、会议、旅行、闲聊五类场景。
- 支持创建会话、提交轮次、结束会话。
- 会话内保存消息、隐藏纠错发现、量化分数和延迟指标。

验收：

- 用户选择场景后能收到匹配场景的 AI 开场。
- 每次用户输入都会生成自然的下一句回应。
- 结束会话可得到完整 report。

### PR-03 Provider 抽象层

状态：已完成

交付：

- 抽象 ASR、TTS、Pronunciation、LLM 四类 provider。
- 添加统一 `ProviderBundle` 工厂。
- mock provider 支持全链路本地运行。

验收：

- session service 不依赖任何具体供应商实现。
- 修改配置即可替换 provider。
- provider 状态接口能展示当前模式。

### PR-04 商用适配器接入边界

状态：已完成

交付：

- OpenAI adapter：支持 Chat Completions 风格调用，用于对话与总结。
- Anthropic adapter：支持 Messages API 风格调用，用于 Claude 对话与总结。
- Azure adapter：保留 ASR 与发音测评统一入口，带密钥校验与部署提示。
- edge-tts adapter：通过 CLI 入口合成语音，便于容器或服务器部署。

验收：

- commercial 模式缺少关键配置时会明确失败。
- LLM、TTS、发音测评调用位置集中在 provider 层。
- mock 与 commercial 不影响业务层代码。

### PR-05 实时语音交互

状态：已完成

交付：

- 前端支持麦克风录音、文本回退输入、AI 语音播放。
- 后端提供 REST 轮次接口与 WebSocket 实时入口。
- UI 中展示会话延迟、发音分、流利度分等即时信号。

验收：

- 用户可以按住录音或输入文本完成一轮对话。
- 对话过程中不展示具体纠错详情。
- AI 回复可以通过浏览器 TTS 回放。

### PR-06 静默纠错与课后总结

状态：已完成

交付：

- 发音错误、语法错误、表达问题在会话中静默累计。
- 结束会话生成总分、CEFR 估计、亮点、问题清单、练习计划。
- 报告语气温和，强调可执行改进。

验收：

- 会话中不打断用户。
- 结束后能看到完整纠错报告。
- 每条纠错都包含原句、建议、原因与严重级别。

### PR-07 前端练习台

状态：已完成

交付：

- 场景选择、级别选择、会话消息、录音按钮、指标面板、报告面板。
- 使用 TypeScript 类型约束 API 数据。
- 响应式布局适配桌面与移动端。

验收：

- 第一屏就是可操作练习台。
- 录音不可用时文本输入仍可完成练习。
- 报告区在结束前不会暴露纠错细节。

### PR-08 部署与质量门槛

状态：已完成

交付：

- Dockerfile、nginx 配置、docker-compose。
- 后端核心 session service 测试。
- README 中明确 mock 与 commercial 部署路径。

验收：

- 本地开发与容器部署路径清晰。
- 业务核心逻辑可测试。
- 正式供应商替换不需要修改前端。

## 部署建议

开发或演示：

- 使用 mock provider。
- 使用 REST 轮次接口即可。
- 浏览器 speechSynthesis 负责 AI 回复播放。

低延迟生产：

- 前端通过 WebSocket 发送 100-300ms 音频 chunk。
- 后端会话编排层并发调度流式 ASR、LLM、TTS。
- LLM 首 token 后立即触发 TTS 分片，减少端到端等待。
- 发音评测可以异步运行，结果进入静默采集池，避免打断对话。

高精准纠错生产：

- 发音测评优先 Azure Speech Pronunciation Assessment。
- 语法与表达纠错由 OpenAI 或 Claude 结合完整上下文生成。
- 报告阶段统一做二次校验，避免单轮误判。
- 保留 per-turn 原始 transcript、分数、建议和模型版本，便于追溯。

## 后续增强

- 增加真实流式 ASR/TTS 的 provider 实现。
- 增加用户历史能力曲线与 spaced repetition 练习计划。
- 增加教师端导出报告。
- 增加多语言场景与自定义情境脚本。
