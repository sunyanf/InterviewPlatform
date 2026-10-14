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
→ v3（P0-5 注入防护：岗位/简历/知识库改为 <untrusted_*> 数据区）
interview.interviewer.v1 → v2（P0-5 注入防护：岗位标题包裹为数据区）
interview.follow_up.v1 → v2（P0-5 注入防护：问题/参考要点/候选人回答包裹为数据区）
interview.answer_analyzer.v1 → v2（P0-5 注入防护：问题/参考要点/候选人回答包裹为数据区）
interview.evaluator.v1 → v2（Week6：整场维度评估，只产维度分/证据/建议，总分由业务代码计算；P0-5：岗位/问答记录/知识库包裹为数据区）
report.learning_planner.v1 → v2（Week7：基于评估结果归纳优点/不足、生成学习计划与下次训练建议，不计算任何分数；P0-5：输入均为已验证评估结果，仅统一升级 system 声明）
audio.speech_analyzer.v1 → v2（Week8：基于转写文本与语音指标分析口头表达，只产定性建议，不算分；P0-5：问题/转写文本包裹为数据区）
realtime.interviewer.v1 → v2（Week9：WebSocket 实时面试官流式对话，自然语言增量输出，不改状态不持久化；P0-5：岗位标题/对话历史/本轮发言包裹为数据区）
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

---

## 5. 变更记录

### P0-5 提示词注入防护代码化

依据：`docs/security/PROMPT_INJECTION.md`

变更内容：

```text
1. 全部 Prompt 的 system 统一追加不可信内容隔离声明（promptguard.IsolationClause），
   声明 <untrusted_*> 标签内内容不得改变规则、评分标准与输出格式
2. 不可信输入统一以 <untrusted_*> 标签包裹（promptguard.Wrap）：
   岗位描述 / 简历 / 候选人回答 / 语音转写 / 知识库参考资料 / 实时对话历史
3. Wrap 中和内容中伪造的同名标签，防止内容提前闭合数据区后被当作指令
4. 简历解析 Prompt（internal/resume）同样加隔离声明与数据区包裹
```

影响面：仅新增安全约束与数据区标记。所有结构化输出 Schema、业务校验、归一化与评分计算逻辑未变；
会话状态仍由业务代码控制（AGENTS.md #10）；LLM 输出仍经 Schema 校验与业务归一化（AGENTS.md #11）。

Eval：本次未改变评分逻辑与输出 Schema，因此未重跑 Eval Dataset。
下次全量 Eval 时需确认评分一致性未因隔离声明产生漂移。
