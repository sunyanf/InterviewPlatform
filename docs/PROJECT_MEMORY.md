# 项目长期记忆

## 1. 项目目的

这是一个 AI 面试与笔试训练平台。

核心目标：

> 帮助用户针对具体岗位进行真实模拟、获得专业反馈，并持续提升能力。

---

## 2. 项目当前策略

当前优先级：

```text
1. 技术岗位面试
2. 公务员结构化面试
3. 文本面试
4. 语音面试
5. 笔试
6. 多模态
```

---

## 3. 当前架构策略

第一阶段采用：

> Modular Monolith

不是因为微服务不好，而是因为：

- 单人开发；
- AI Coding 迭代快；
- 早期需求变化大；
- 核心挑战在 AI 产品逻辑而不是服务数量。

---

## 4. 长期服务边界

计划形成：

```text
User
Job
Resume
Interview
Agent
Knowledge
Evaluation
Report
Learning
Media
```

是否拆成独立服务，应以后续实际负载和团队规模决定。

---

## 5. 核心领域模型

```text
User
  ↓
Resume
  ↓
Job
  ↓
InterviewSession
  ↓
InterviewQuestion
  ↓
InterviewAnswer
  ↓
EvaluationResult
  ↓
InterviewReport
  ↓
UserCapability
  ↓
LearningPlan
```

---

## 6. AI 核心模型

Agent 不应该是一个超级 Agent。

拆分为：

```text
Question Planner
Interviewer
Follow-up
Answer Analyzer
Evaluator
Coach
```

---

## 7. 核心约束

1. LLM 不控制核心状态。
2. 用户声明默认不等于事实。
3. AI 输出必须结构化和验证。
4. Prompt 必须版本化。
5. 评分必须有 Rubric 和 Evidence。
6. RAG 必须记录来源。
7. 高风险变化必须人工确认。

---

## 8. 项目决策习惯

所有长期架构决策进入：

```text
docs/adr/
```

所有长期产品事实进入：

```text
docs/PROJECT_MEMORY.md
```

所有临时实现任务进入：

```text
docs/tasks/
```
