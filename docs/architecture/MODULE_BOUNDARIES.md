# 模块边界

## User

负责：

- 用户；
- 认证；
- 个人资料。

不负责：

- 面试评分；
- RAG。

## Job

负责：

- 岗位；
- JD；
- 岗位要求。

## Resume

负责：

- 文件；
- 简历解析；
- 结构化经历。

## Interview

负责：

- Session；
- Question；
- Answer；
- 状态机。

## Agent

负责：

- AI 编排；
- Prompt；
- LLM；
- Tool；
- Agent Context。

## Knowledge

负责：

- Document；
- Chunk；
- Embedding；
- Retrieval。

## Evaluation

负责：

- Rubric；
- Evidence；
- Score。

## Report

负责：

- 报告；
- 汇总。

## Learning

负责：

- 能力画像；
- 学习计划。

---

# 禁止

模块之间禁止直接：

```text
读取对方内部数据库实现细节
绕过 Service 修改对方状态
```
