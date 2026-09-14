# ADR-0003：第一阶段使用 PostgreSQL + pgvector

## 状态

Accepted

## 决策

第一版使用 PostgreSQL 作为：

```text
关系数据库
+
全文检索
+
向量存储
```

## 原因

- 数据规模可控；
- 结构简单；
- 运维成本低；
- AI Coding 上下文集中；
- 足够支持 MVP。

数据量明显增长再评估独立向量数据库。
