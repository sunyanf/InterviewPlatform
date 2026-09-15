# API 规范

## 1. 版本

统一：

```text
/api/v1
```

---

# 2. 认证

```text
POST /api/v1/auth/register   注册即登录，返回令牌对
POST /api/v1/auth/login      登录，返回令牌对
POST /api/v1/auth/refresh    旋转 refresh token，换取新令牌对
POST /api/v1/auth/logout     吊销 refresh token（幂等）
GET  /api/v1/me              当前用户资料（需 Bearer access token）
```

登录/注册/刷新响应体：

```json
{
  "token": "<access JWT，2h>",
  "refresh_token": "<256bit 随机串，30d；数据库仅存其 SHA-256>",
  "expires_in": 7200,
  "user": { "id": "...", "email": "...", "nickname": "..." }
}
```

刷新与安全策略：

- access token 过期后用 refresh token 调 `/auth/refresh`；**refresh token 一次性旋转**，旧 token 立即吊销；
- 已吊销的 refresh token 被再次使用视为被盗：该用户全部活跃 token 被吊销，需重新登录；
- 登录/注册/刷新按 IP 限流（默认 10 次/分钟，超限 429 `RATE_LIMITED`）；其他 API 按用户限流（默认 120 次/分钟）；
- JSON 请求体上限 1MiB（413 `PAYLOAD_TOO_LARGE`），简历/录音上传上限 12MiB；
- 跨域由 `ALLOWED_ORIGINS` 精确白名单控制（生产必须配置）。

---

# 3. 岗位

```text
GET  /api/v1/jobs
GET  /api/v1/jobs/:id
POST /api/v1/jobs
```

---

# 4. 简历

```text
POST /api/v1/resumes
GET  /api/v1/resumes/:id
POST /api/v1/resumes/:id/parse   提交解析任务（异步，返回 202 + task_id）
```

---

# 5. 面试

```text
POST /api/v1/interviews
POST /api/v1/interviews/:id/start
GET  /api/v1/interviews/:id
POST /api/v1/interviews/:id/answer
POST /api/v1/interviews/:id/finish
```

`POST /interviews/:id/answer` 响应包含 Agent 分析与追问建议：

```json
{
  "data": {
    "answer": { "id": "...", "text_content": "..." },
    "analysis": {
      "claims": ["..."],
      "correct_points": ["..."],
      "wrong_points": ["..."],
      "missing_points": ["..."],
      "knowledge_gaps": ["..."],
      "prompt_version": "interview.answer_analyzer.v1",
      "model": "..."
    },
    "follow_up": { "id": "...", "question_type": "follow_up", "question": "..." }
  }
}
```

`analysis` / `follow_up` 为 Agent 建议，均由业务代码决定是否落库；LLM 不拥有会话状态（ADR-0004）。

---

# 5.5 知识库（RAG）

需要鉴权。

```text
POST   /api/v1/knowledge/documents   摄取文档（分块 + 向量化 + 入库）
GET    /api/v1/knowledge/documents   文档列表（分页）
GET    /api/v1/knowledge/documents/:id
DELETE /api/v1/knowledge/documents/:id   分块级联删除
POST   /api/v1/knowledge/search      混合检索（向量 + FTS + 关键词，RRF 融合）
```

`POST /knowledge/documents` 请求体：

```json
{
  "title": "Go 并发面试要点",
  "content": "文档正文…",
  "source": "official-docs",
  "source_url": "https://go.dev/doc/",
  "source_type": "official",
  "effective_from": "2026-01-01T00:00:00Z",
  "effective_to": null,
  "metadata": { "domain": "go", "topic": "concurrency", "content_type": "interview" }
}
```

约束：

- `source` 必填（禁止来源不明的知识，AGENTS.md #14）；
- `source_type` ∈ manual / official / interview_bank / docs；
- `effective_from` / `effective_to` 为 RFC3339，可选；过期知识不参与检索（AGENTS.md #16）。

`POST /knowledge/search` 请求体与响应：

```json
{
  "query": "goroutine 调度原理",
  "domain": "go",
  "top_k": 5,
  "results": [
    {
      "chunk_id": "...",
      "document_id": "...",
      "seq": 1,
      "content": "…",
      "score": 0.0328,
      "channels": ["vector", "fts", "keyword"],
      "metadata": { "source": "official-docs", "domain": "go" }
    }
  ]
}
```

无结果时返回空 `results`，调用方应标记 `no_evidence`，不得伪造知识来源（docs/RAG.md #7）。

---

# 5.6 评估（Evaluation）

需要鉴权。仅对 `COMPLETED` 状态的面试可评估；同一会话重跑覆盖。

```text
POST /api/v1/evaluations/sessions/:sessionID   提交评估任务（异步 202 + task_id；LLM 维度评分 → 确定性总分）
GET  /api/v1/evaluations/sessions/:sessionID   查询会话评估（未评估 404）
GET  /api/v1/evaluations                       当前用户评估列表
```

> POST 为**异步**：HTTP 202 返回 `task_id`，轮询 `GET /api/v1/tasks/:taskID` 至 succeeded 后再 GET 评估结果（见 5.8）。

评估基于证据（AGENTS.md #15）：LLM 只输出维度分（0-100）+ 证据（引用回答原文）+ 改进建议；
**总分由业务代码按 Rubric 权重确定性计算**：`correctness 0.30 + depth 0.25 + logic 0.25 + communication 0.20`。

评估失败可重新触发（Upsert 覆盖）；无回答的面试返回 409；跨用户访问返回 403。

响应示例：

```json
{
  "id": "...",
  "session_id": "...",
  "dimensions": {"correctness": 70, "depth": 60, "logic": 75, "communication": 80},
  "evidence": [{"question_seq": 1, "issue": "…", "evidence": "…回答原文…", "reference": "go-official-docs"}],
  "recommendations": ["…", "…"],
  "rubric": {"dimensions": [{"name": "correctness", "label": "技术正确性", "weight": 0.30}]},
  "total_score": 70.75,
  "prompt_version": "interview.evaluator.v1",
  "model": "mock"
}
```

---

# 5.7 报告（Report）

需要鉴权。仅对 `COMPLETED` 且**已评估**的面试可生成报告（未评估 404 `EVALUATION_REQUIRED`）；同一会话重跑覆盖。

```text
POST /api/v1/reports/sessions/:sessionID   提交报告生成任务（异步 202 + task_id）
GET  /api/v1/reports/sessions/:sessionID   查询会话报告（未生成 404）
GET  /api/v1/reports                       当前用户报告列表
```

数据来源与确定性边界（AGENTS.md #15）：

```text
总分 / 能力维度      → 复制评估结果（确定性）
历史对比             → 近 5 场同类型面试评估均值 + delta（确定性计算）
知识缺口             → 从已存储的回答分析确定性聚合去重（上限 10）
优点 / 不足 / 学习计划 → LLM（report.learning_planner.v1，只产建议不算分）
```

响应示例（节选）：

```json
{
  "session_id": "...",
  "total_score": 70.75,
  "capability_profile": {
    "dimensions": {"correctness": 70, "depth": 60, "logic": 75, "communication": 80},
    "total_score": 70.75,
    "history": {
      "compared_count": 2,
      "avg_total_score": 65.5,
      "avg_dimensions": {"correctness": 62.5},
      "delta_total_score": 5.25,
      "delta_dimensions": {"correctness": 7.5}
    }
  },
  "strengths": ["..."],
  "weaknesses": ["..."],
  "knowledge_gaps": ["channel 关闭语义", "GMP 调度模型"],
  "learning_plan": {
    "focus_areas": [{"topic": "...", "reason": "...", "suggestions": ["..."]}],
    "next_training": {"focus": "...", "suggested_question_type": "technical", "suggested_difficulty": "medium", "suggested_topics": ["..."]}
  }
}
```

---

# 5.8 异步任务（Task）

长耗时 LLM 操作（简历解析、面试评估、报告生成）统一走异步任务：POST 端点立即返回 **HTTP 202**，由后台 worker 执行，客户端凭 `task_id` 轮询状态。

```text
GET /api/v1/tasks/:taskID   查询任务状态（仅可查询本人提交的任务，跨用户 404）
```

202 响应体：

```json
{
  "code": "ACCEPTED",
  "message": "accepted",
  "data": { "task_id": "01J...", "status": "pending" }
}
```

任务状态响应：

```json
{
  "id": "01J...",
  "type": "session_evaluation",
  "status": "succeeded",
  "attempts": 1,
  "max_attempts": 3,
  "last_error": "",
  "created_at": "...",
  "updated_at": "..."
}
```

状态机：`pending → running → succeeded | failed`。失败按指数退避自动重试（10s/20s/40s，上限 2 分钟，共 max_attempts 次），`failed` 时读 `last_error`；达到终态前建议每 1~2 秒轮询一次。

幂等：同一业务对象（如同一份简历、同一场会话）在 pending/running 期间重复提交，返回同一个进行中任务；终态后重新提交则创建新任务（重跑覆盖旧产物）。任务成功后，再 GET 对应业务资源获取产物。

任务类型：

```text
resume_parse         简历解析（payload: user_id, resume_id）
session_evaluation   面试评估（payload: user_id, session_id）
report_generation    报告生成（payload: user_id, session_id）
```

---

# 5.9 语音（Audio）

需要鉴权。答案录音挂在题目上（一题一份，重传覆盖）；仅 `RUNNING` 状态会话可上传；转写与分析可重跑。

```text
POST /api/v1/answers/:questionID/audio             上传录音（multipart: file + duration_ms，≤10MB，wav/mp3/m4a/webm/ogg）
POST /api/v1/answers/:questionID/audio/transcribe  ASR 转写
POST /api/v1/answers/:questionID/audio/analyze     语音表达分析
GET  /api/v1/answers/:questionID/audio             查询语音详情（含指标 + 15 分钟临时下载 URL）
```

错误语义：跨用户 403；会话非 RUNNING 上传 400 `SESSION_NOT_RUNNING`；格式不支持 400 `INVALID_FORMAT`；未上传 404 `AUDIO_NOT_FOUND`；未转写先分析 400 `NOT_TRANSCRIBED`；转写为空 400 `ASR_EMPTY_TRANSCRIPT`。

前端语音作答链路（`interview/VoiceRecorder.tsx`）：

```text
MediaRecorder（webm/opus，Safari 回退 mp4）
  → POST /answers/{id}/audio（file + duration_ms）
  → POST /answers/{id}/audio/transcribe
  → 转写文本回填回答输入框（候选人可修改）
  → 随文字答案一起 POST /interviews/{id}/answer
```

录音仅用于生成文字答案，提交的仍以文本为准（语音不是最终事实）；
麦克风权限被拒/无设备时展示明确提示，不阻断文字作答。

语音量化指标由业务代码确定性计算（语速 = 有效字符/分钟，口头禅计数）；
表达分析（`audio.speech_analyzer.v1`）只产定性建议，不计算分数；日志不输出音频与转写内容（AGENTS.md #17）。

响应示例（GET）：

```json
{
  "audio": {
    "session_id": "...",
    "question_id": "...",
    "format": "wav",
    "size_bytes": 102400,
    "duration_ms": 61000,
    "transcript": "…转写文本…",
    "language": "zh",
    "status": "analyzed",
    "asr_provider": "mock",
    "analysis": {"strengths": ["…"], "issues": ["…"], "suggestions": ["…"]}
  },
  "metrics": {
    "chars_per_minute": 240.0,
    "pace": "normal",
    "filler_count": 5,
    "filler_detail": {"嗯": 3, "那个": 2}
  },
  "download_url": "https://…presigned…",
  "download_expire_seconds": 900
}
```

---

# 5.9 面试官语音（TTS）

需要鉴权。将面试官侧文本（开场白、面试问题、实时对话回复）合成为语音。
音频不直接返回二进制，而是存入对象存储后下发 **15 分钟预签名 URL**；
合成结果按 `provider + model + voice + format + 文本` 内容寻址缓存，重复请求不再次调用付费 Provider。

```text
GET /api/v1/interviews/{id}/speech/opening                          合成开场白语音
GET /api/v1/interviews/{id}/questions/{questionID}/speech           合成指定问题语音
```

错误语义：跨用户 403；开场白不存在 404 `OPENING_NOT_AVAILABLE`；问题不属于该面试 404 `QUESTION_NOT_FOUND`；
文本超长 400 `SPEECH_TEXT_TOO_LONG`（上限 2000 字）；Provider 失败 502 `TTS_SYNTHESIZE_FAILED`。

响应示例：

```json
{
  "text": "你好，欢迎参加本次面试……",
  "download_url": "https://…presigned…",
  "format": "mp3",
  "content_type": "audio/mpeg",
  "size_bytes": 24576,
  "cached": false,
  "download_expire_seconds": 900
}
```

配置（环境变量）：`TTS_PROVIDER`（mock/openai，默认 mock 输出确定性 400ms WAV 正弦波）、
`TTS_API_KEY`、`TTS_BASE_URL`、`TTS_MODEL`（默认 tts-1）、`TTS_VOICE`（默认 alloy）、
`TTS_FORMAT`（默认 mp3；mock 固定输出 wav）。

---

# 6. WebSocket 实时面试

```text
GET /api/v1/interviews/{id}/ws?token={JWT}
```

浏览器 WebSocket API 无法设置 `Authorization` 头，JWT 通过 query 参数 `token` 传递。
鉴权失败 / 会话不存在 / 跨用户访问在协议升级前以普通 HTTP 状态码返回（401 / 404 / 403）。
同一会话只允许一个活跃连接：新连接顶替旧连接（旧连接收到 close frame，code=4000）。
所有帧均为 JSON 文本，统一信封：

```json
{ "type": "消息类型", "data": { } }
```

## 6.1 连接与重连

连接建立后服务端立即推送 `snapshot`（会话全量状态，结构同 `GET /interviews/{id}`）。
断线重连后重新连接即可再次收到快照，客户端据此全量恢复，无需维护事件游标。

心跳：客户端可发送 `{"type":"ping"}`，服务端回复 `{"type":"pong"}`；
此外服务端每 50 秒发送 WebSocket 协议层 Ping，60 秒无响应判定死连接。

## 6.2 客户端 → 服务端

### 提交结构化回答（与 REST 提交回答等价，结果分阶段推送）

```json
{
  "type": "answer",
  "data": { "question_id": "...", "text_content": "...", "duration_ms": 3000 }
}
```

### 实时面试官对话（token 流式回复；仅 RUNNING 会话，单条 ≤ 2000 字符）

```json
{
  "type": "chat",
  "data": {
    "message": "能给点提示吗？",
    "tts": true,
    "history": [
      { "role": "user", "content": "上一轮发言" },
      { "role": "assistant", "content": "上一轮面试官回复" }
    ]
  }
}
```

`history` 为可选的本连接内对话历史（最多保留 20 条，服务端只接受 user/assistant 角色）。
实时对话不改变任何会话状态、不持久化、不进入评估。
可选 `tts: true` 请求对整段回复合成语音：所有 `chat_delta` 推送完毕后、`chat_done` 之前，
额外下发一条 `chat_speech` 事件（含 15 分钟预签名下载 URL）；合成失败只发 `error` 事件，文字流仍正常以 `chat_done` 结束。

### 心跳

```json
{ "type": "ping" }
```

## 6.3 服务端 → 客户端

| type | data | 时机 |
|---|---|---|
| `snapshot` | Session 全量状态 | 连接 / 重连 |
| `answer_saved` | Answer | answer 处理：回答已落库 |
| `analysis` | AnswerAnalysis | answer 处理：AI 分析完成（可为空不推） |
| `follow_up` | Question | answer 处理：产生追问问题（可为空不推） |
| `chat_delta` | `{"delta":"..."}` | 实时对话 token 增量 |
| `chat_speech` | `{"download_url":"…","format":"mp3","content_type":"audio/mpeg","size_bytes":24576,"cached":false,"download_expire_seconds":900}` | chat 请求 `tts:true`：回复语音合成结果（在 chat_done 前下发） |
| `chat_done` | `{}` | 实时对话流结束 |
| `pong` | `{}` | 响应客户端 ping |
| `error` | `{"code":"...","message":"..."}` | 业务错误（连接保持，可继续） |

`error` 常见 code：`BAD_REQUEST`、`EMPTY_MESSAGE`、`MESSAGE_TOO_LONG`、
`SESSION_NOT_RUNNING`、`ALREADY_ANSWERED`、`INVALID_STATE_TRANSITION`、
`LLM_STREAM_ERROR`、`UNKNOWN_MESSAGE_TYPE`、
`TTS_UNAVAILABLE`（服务未配置合成能力）、`TTS_EMPTY_REPLY`、`TTS_SYNTHESIZE_FAILED`。

close code：`4000` 被新连接顶替；`4001` 服务端主动关闭（如发送缓冲溢出）。

---

# 7. 错误格式

统一：

```json
{
  "code": "INTERVIEW_SESSION_NOT_FOUND",
  "message": "面试不存在",
  "request_id": "..."
}
```

不要把内部错误栈返回给用户。
