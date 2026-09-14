# Prompt 注册表

## 1. 原则

Prompt 必须像代码一样管理。

---

## 2. 命名

```text
领域.能力.版本
```

例如：

```text
interview.question_planner.v1 → v2（Week5：出题增加 RAG 知识库参考资料，无资料时行为与 v1 一致）
interview.interviewer.v1
interview.follow_up.v1
interview.answer_analyzer.v1
interview.evaluator.v1（Week6：整场维度评估，只产维度分/证据/建议，总分由业务代码计算）
evaluation.evaluator.v1
learning.coach.v1
```

---

## 3. 每个 Prompt 必须有

```text
名称
版本
用途
输入
输出 Schema
系统约束
风险
Eval Dataset
```

---

## 4. 修改流程

```text
修改 Prompt
↓
更新版本
↓
运行 Eval
↓
比较结果
↓
记录结论
```
