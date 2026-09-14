# AI 面试模拟平台

一个面向求职者和考试人群的 AI 面试 / 笔试训练平台。

项目目标不是简单实现“一个聊天机器人”，而是建立一套能够：

```text
理解岗位
↓
理解用户简历
↓
检索岗位知识与评分标准
↓
规划面试
↓
动态追问
↓
接收文字 / 语音 / 图片
↓
分析回答
↓
专业评分
↓
生成报告
↓
形成能力画像
↓
生成学习计划
```

的 AI Agent 系统。

---

## 1. 产品定位

目标方向：

- 技术岗位
- 产品 / 运营等职场岗位
- 公务员及结构化面试
- 笔试训练

第一阶段优先：

> **技术岗位 + 公务员结构化面试**

---

## 2. 核心价值

不是：

> “让 AI 陪用户聊天。”

而是：

> **通过真实岗位、题目、Rubric、用户回答和历史表现，持续评估用户能力，并给出可执行的改进路径。**

---

## 3. 核心闭环

```text
岗位
+
简历
+
目标
      ↓
面试计划
      ↓
AI Interviewer
      ↓
用户回答
      ↓
Answer Analyzer
      ↓
Evaluator
      ↓
Report
      ↓
Capability Profile
      ↓
Learning Plan
      ↓
下一次训练
```

---

## 4. 技术路线

第一阶段：

```text
Go
PostgreSQL
pgvector
Redis
MinIO / S3
WebSocket
LLM
Embedding
ASR / TTS
OpenTelemetry
Prometheus
Docker Compose
Next.js
```

第二阶段：

```text
Kafka
Kubernetes
更强的 Eval 系统
更多岗位
笔试
多模态
```

---

## 5. 架构原则

### Modular Monolith 优先

代码从第一天按模块边界设计：

```text
user
job
resume
interview
question
agent
knowledge
evaluation
report
learning
media
```

部署第一版先保持一个后端服务。

---

## 6. AI 原则

```text
代码负责确定性
LLM 负责非确定性
```

例如：

| 类型 | 责任方 |
|---|---|
| 面试状态 | Go |
| 面试计时 | Go |
| 权限 | Go |
| 最终分数 | Go |
| 数据写入 | Go |
| 问题生成 | LLM |
| 回答理解 | LLM |
| 反馈总结 | LLM |
| 学习建议 | LLM |

---

## 7. 文档入口

### 人类开发者先读

```text
README.md
docs/PRODUCT.md
docs/ARCHITECTURE.md
docs/DEVELOPMENT.md
```

### AI Coding Agent 先读

```text
AGENTS.md
docs/PROJECT_MEMORY.md
docs/ai-coding/CONTEXT.md
docs/ai-coding/WORKFLOW.md
```

### 开发具体模块

优先读取目标目录附近的：

```text
AGENTS.md
```

---

## 8. 当前阶段

项目采用：

```text
MVP
↓
V1
↓
V2
↓
长期产品
```

不要为了未来需求提前实现复杂系统。
