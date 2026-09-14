# 发布规范

## 1. 发布前

检查：

```text
测试
Migration
Config
Secret
AI Eval
Observability
```

---

# 2. AI 变更

若修改：

```text
Prompt
Model
Rubric
Schema
RAG
```

必须查看：

```text
Eval
Cost
Latency
Regression
```

---

# 3. 数据库

生产 Migration 必须：

```text
向前兼容
可回滚
有明确影响范围
```

---

# 4. 回滚

至少明确：

```text
应用回滚
Prompt 回滚
Model 回滚
Schema 回滚
Migration 回滚策略
```
