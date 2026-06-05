# T2T ai英语口语练习工具

T2T 是一款面向真实场景的英语口语练习工具。当前版本已经包含 Go + Gin 后端、React + TypeScript + Vite 前端、可本地全链路运行的模拟适配器，以及可切换到商用供应商的适配器边界。


## 技术选型

| 层级 | 选型 | 原因 |
| --- | --- | --- |
| 后端 | Go + Gin | 轻量、并发性能稳定、部署简单，适合低延迟语音编排服务。 |
| 前端 | React + TypeScript + Vite | 交互开发效率高，适合构建录音、会话、评分报告等复杂状态 UI。 |
| ASR | Provider 抽象，默认 mock | 方便本地演示；正式环境可切 Azure Speech 或其他流式 ASR。 |
| TTS | Provider 抽象，默认 mock，正式 edge-tts | mock 模式用浏览器 speechSynthesis 回退；正式环境用 edge-tts 生成自然语音。 |
| 发音评测 | Provider 抽象，默认 mock，正式 Azure Speech Pronunciation Assessment | Azure 发音测评在发音、流利度、完整度等维度成熟，适合量化反馈。 |
| LLM | Provider 抽象，支持 OpenAI / Anthropic Claude | OpenAI 用于低延迟对话与纠错，Claude 可用于高质量总结报告或备用模型。 |
