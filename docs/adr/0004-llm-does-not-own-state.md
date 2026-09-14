# ADR-0004：LLM 不拥有业务状态

## 状态

Accepted

## 决策

LLM 可以提出：

```text
next_question
follow_up
evaluation
recommendation
```

但不能直接决定：

```text
session status
permission
transaction
final score
resource usage
```

## 原因

LLM 是非确定系统。

业务状态必须可预测、可测试、可恢复。
