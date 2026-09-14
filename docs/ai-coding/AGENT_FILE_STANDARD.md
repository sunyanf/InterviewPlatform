# AGENTS.md 分层标准

## 根目录

```text
/AGENTS.md
```

负责：

- 最高原则；
- 安全；
- 规则优先级；
- 全局禁止事项；
- 全局 Done Definition。

---

# backend

```text
/backend/AGENTS.md
```

负责：

- Go；
- 分层；
- API；
- DB；
- 测试；
- 日志。

---

# Agent 模块

```text
/backend/internal/agent/AGENTS.md
```

负责：

- Agent 编排；
- LLM Interface；
- State；
- Tool；
- Prompt；
- Schema。

---

# Interview 模块

```text
/backend/internal/interview/AGENTS.md
```

负责：

- Session；
- Question；
- Answer；
- 状态机；
- 并发。

---

# Knowledge 模块

```text
/backend/internal/knowledge/AGENTS.md
```

负责：

- Document；
- Chunk；
- Embedding；
- Retrieval；
- Source。

---

# Prompt

```text
/prompts/AGENTS.md
```

负责：

- Prompt 版本；
- 命名；
- 输入输出；
- 测试。

---

# Eval

```text
/evals/AGENTS.md
```

负责：

- Dataset；
- Rubric；
- Regression；
- Score。

---

# Migration

```text
/migrations/AGENTS.md
```

负责：

- Migration 命名；
- 向前；
- 回滚；
- 危险操作。
