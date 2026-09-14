# 项目通用规则

## 1. 架构

优先 Modular Monolith。

## 2. 数据

PostgreSQL 为主数据库。

Redis 用于：

- cache；
- session；
- rate limit；
- queue。

## 3. AI

模型调用通过统一 Provider Interface。

## 4. RAG

第一版 PostgreSQL + pgvector。

## 5. 实时

WebSocket 用于面试实时通信。

## 6. 文件

MinIO / S3 私有对象存储。

## 7. 可观测性

OpenTelemetry + Prometheus。

## 8. 版本

API：

```text
/api/v1
```

Prompt、Rubric、Schema 必须版本化。
