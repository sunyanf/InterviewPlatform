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

# 5.6 评估（Evaluation）

需要鉴权。仅对 `COMPLETED` 状态的面试可评估；同一会话重跑覆盖。

```text
POST /api/v1/evaluations/sessions/:sessionID   触发评估（LLM 维度评分 → 确定性总分）
GET  /api/v1/evaluations/sessions/:sessionID   查询会话评估（未评估 404）
GET  /api/v1/evaluations                       当前用户评估列表
```

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
POST /api/v1/reports/sessions/:sessionID   触发生成报告
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
