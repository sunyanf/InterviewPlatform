# AI Evaluation 设计

## 1. 为什么必须单独设计 Evaluation

如果没有 Evaluation：

```text
AI 说 80 分
```

无法知道是否真的合理。

本项目需要建立：

> 可重复、可比较、可回归的 AI 评估体系。

---

# 2. Rubric

技术岗位示例：

```text
技术正确性 30%
技术深度 20%
系统思维 20%
项目经验 20%
表达 10%
```

公务员示例：

```text
内容
逻辑
综合分析
表达
应变
```

不同岗位使用不同 Rubric。

---

# 3. Evidence

每个扣分项应尽量有证据：

```json
{
  "issue": "没有解释缓存一致性",
  "evidence": "...用户原回答片段...",
  "reference": "knowledge_chunk_id"
}
```

---

# 4. Score

流程：

```text
回答
↓
Evidence Extraction
↓
Dimension Evaluation
↓
Business Score Calculator
↓
Final Score
```

---

# 5. Eval Dataset

至少维护：

```text
question
answer
expected_score
expected_dimension
expected_feedback
```

数据集分：

```text
gold
regression
adversarial
edge_cases
```

---

# 6. AI 输出质量指标

推荐关注：

```text
JSON Valid Rate
Rubric Agreement
Evidence Accuracy
Follow-up Relevance
Hallucination Rate
RAG Groundedness
```

---

# 7. Prompt 修改规则

Prompt 变化后：

1. 重新运行核心 Eval；
2. 比较版本差异；
3. 记录退化；
4. 决定是否发布。
