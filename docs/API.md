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
