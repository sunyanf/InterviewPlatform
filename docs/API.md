# API 规范

## 1. 版本

统一：

```text
/api/v1
```

---

# 2. 认证

```text
POST /api/v1/auth/register
POST /api/v1/auth/login
GET  /api/v1/me
```

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
POST /api/v1/resumes/:id/parse
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

# 6. WebSocket

```text
/ws/v1/interviews/{session_id}
```

消息：

```json
{
  "type": "user_message",
  "content": "..."
}
```

服务端消息：

```json
{
  "type": "interviewer_message",
  "content": "..."
}
```

状态：

```json
{
  "type": "state",
  "status": "RUNNING",
  "question_no": 4,
  "remaining_seconds": 1020
}
```

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
