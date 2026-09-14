# AI 数据 Contract

## 1. 原则

AI 系统中：

> 输入、输出、Schema、版本必须明确。

---

# 2. 面试题生成

```json
{
  "question": "...",
  "type": "technical",
  "difficulty": "senior",
  "target_capabilities": ["go.concurrency"],
  "expected_points": ["..."]
}
```

---

# 3. Follow-up

```json
{
  "should_follow_up": true,
  "reason": "...",
  "question": "...",
  "target_gap": "..."
}
```

---

# 4. Answer Analysis

```json
{
  "claims": [],
  "correct_points": [],
  "wrong_points": [],
  "missing_points": [],
  "knowledge_gaps": []
}
```

---

# 5. Evaluation

```json
{
  "dimensions": {
    "correctness": 0,
    "depth": 0,
    "logic": 0,
    "communication": 0
  },
  "evidence": [],
  "recommendations": []
}
```

---

# 6. Contract 版本

任何 Schema 变更都必须考虑：

```text
backward compatibility
prompt compatibility
database compatibility
eval compatibility
```
