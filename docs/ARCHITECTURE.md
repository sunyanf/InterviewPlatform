# 系统总体架构

## 1. 总体图

```text
Web / App
   │
   ▼
API / WebSocket
   │
   ▼
Go Application
   │
   ├── User
   ├── Job
   ├── Resume
   ├── Interview
   ├── Agent
   ├── Knowledge
   ├── Evaluation
   ├── Report
   └── Learning
   │
   ├──────────────┬─────────────┐
   ▼              ▼             ▼
PostgreSQL      Redis         Object Storage
   │
pgvector
```

---

# 2. 模块关系

```text
User
 ↓
Resume
 ↓
Job
 ↓
Interview
 ↓
Agent
 ├── Knowledge
 └── LLM
 ↓
Evaluation
 ↓
Report
 ↓
Learning
```

---

# 3. 模块边界

模块必须通过自己的 Service 接口访问业务能力。

禁止：

```text
interview
直接修改
evaluation 的数据库表
```

正确：

```text
interview
 ↓
evaluation service interface
```

---

# 4. 技术原则

## 第一版

一个 Go 进程。

内部模块化。

## 后续

只有当：

- 负载；
- 团队；
- 发布；
- 故障隔离；

真的需要时，再拆服务。

---

# 5. AI 请求链

```text
HTTP / WS
 ↓
Interview Service
 ↓
Agent Orchestrator
 ↓
RAG Retrieval
 ↓
LLM
 ↓
Schema Validation
 ↓
Business Validation
 ↓
Interview State
 ↓
Response
```

---

# 6. 异步链路

后续：

```text
InterviewFinished
 ↓
Event Bus / Kafka
 ↓
Evaluation Worker
 ↓
Report Worker
 ↓
Learning Worker
```

MVP 可以使用进程内 Worker 或 Redis Queue。
