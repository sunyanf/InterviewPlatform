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

当前数据集位于 `evals/datasets/evaluator/`：

- `golden.jsonl`：22 条人工标注样本（golden + edge_cases：空回答、跑题、偏科、长篇空洞、临界分等），
  每条含 `expected_dimensions` 与按 Rubric 权重确定性算出的 `expected_total`；
- `adversarial.jsonl`：17 条契约异常样本（空输出、非 JSON、缺字段、错类型、越界分、markdown 包裹、夹带解释文字等），
  每条用 `scenario` 引用脚本化 Provider 的固定输出，`expect_valid` 标注业务层应接受还是拒绝。

回归工具 `backend/cmd/evalrunner`：

```bash
# mock 模式（CI 默认执行）：golden 结构化契约回归 + adversarial 异常回归
cd backend && go run ./cmd/evalrunner

# 真实 Provider 评分一致性回归（阶段 B 配置密钥后）
LLM_PROVIDER=openai LLM_API_KEY=... LLM_MODEL=gpt-4o-mini \
  go run ./cmd/evalrunner -agreement -tolerance 15 -min-agreement 0.8 \
    -out ../evals/results/evaluator-$(date +%F).json
```

报告字段：`passed/failed`、失败原因、每维度偏差、`agreement_rate`（容差内通过率）
与 `mean_abs_error`。退出码：契约失败或一致率低于阈值返回非 0。
mock 输出与答案质量无关，`-agreement` 在 mock 下会被忽略。Go 层回归测试见
`backend/internal/evalrun/evalrun_test.go`，CI 已接入。

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
