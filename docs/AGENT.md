# Agent 系统设计

## 1. Agent 不等于一个大 Prompt

采用多个职责明确的 Agent / Agent Role：

```text
Question Planner
Interviewer
Follow-up
Answer Analyzer
Evaluator
Coach
```

---

# 2. Question Planner

输入：

```text
岗位
JD
简历
面试类型
目标能力
难度
时长
```

输出：

```text
问题计划
能力覆盖
难度分布
```

---

# 3. Interviewer

负责：

- 询问；
- 澄清；
- 引导；
- 控制面试语气。

不负责：

- 修改数据库状态；
- 修改最终分数；
- 绕过权限。

---

# 4. Follow-up

目标：

> 根据当前回答发现信息缺口，并生成最有价值的下一问。

输入：

```text
question
answer
expected_points
covered_points
current_state
```

输出：

```json
{
  "should_follow_up": true,
  "reason": "...",
  "question": "..."
}
```

---

# 5. Answer Analyzer

负责提取：

```text
facts
claims
correct_points
wrong_points
missing_points
knowledge_gaps
```

---

# 6. Evaluator

必须使用：

```text
Rubric
+
Reference Knowledge
+
Evidence
```

输出维度分，而不是只输出一个总分。

---

# 7. Coach

负责：

- 总结；
- 提炼弱项；
- 生成学习建议；
- 推荐下一次训练重点。

---

# 8. Agent 编排

优先采用：

```text
代码控制流程
+
模型提供决策候选
```

而不是：

```text
LLM 自由调用所有工具
```

---

# 9. Agent 状态

Agent 状态分两种：

### 业务状态

由 Go 控制。

### 推理上下文

可以由 Agent 管理，但最终需要映射回业务模型时必须经过业务校验。
