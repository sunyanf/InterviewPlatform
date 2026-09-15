# RAG 方案

## 1. 目标

让 AI 面试基于：

- 岗位要求；
- 面试题；
- 知识库；
- 评分标准；
- 用户简历；

进行有依据的提问和评价。

---

# 2. Pipeline

```text
Document
 ↓
Parse
 ↓
Clean
 ↓
Chunk
 ↓
Metadata
 ↓
Embedding
 ↓
pgvector
```

---

# 3. Retrieval

默认：

```text
Query
 ↓
Metadata Filter
 ↓
Keyword Search
 ↓
Vector Search
 ↓
Hybrid Merge
 ↓
Rerank
 ↓
Context
```

---

# 4. Metadata

示例：

```json
{
  "domain": "go",
  "topic": "concurrency",
  "difficulty": "senior",
  "content_type": "interview",
  "source": "..."
}
```

---

# 5. Chunk

Chunk 必须：

- 语义完整；
- 尽量避免跨多个主题；
- 带 source 和 version；
- 保存 document_id。

---

# 6. 召回结果

RAG 返回结果应该保留：

```text
chunk_id
document_id
score
source
metadata
content
```

---

# 7. 无结果策略

如果没有高质量结果：

> 不应该伪造“知识来源”。

可以：

```text
降低置信度
标记 no_evidence
让模型明确说明依据不足
```

---

# 8. 知识版本

知识更新以后不要无脑覆盖。

推荐：

```text
document version
chunk version
effective_from
effective_to
```

---

# 9. 第一版技术选择

优先：

```text
PostgreSQL
+
pgvector
+
PostgreSQL Full Text Search
```

后续数据规模明显增长再考虑独立向量数据库。

---

# 10. MVP 实现现状

## 10.1 知识导入

`backend/cmd/ingest` 将本地 `.md/.txt` 文件批量导入（每个文件一个文档）：

```bash
go run ./cmd/ingest \
  -dir ./seed/knowledge \
  -source "2026 公务员题库" \
  -source-type interview_bank \
  -source-url https://example.com/bank \
  -effective-from 2026-01-01T00:00:00Z \
  -domain civil_service -topic 综合分析
```

- `-source` 必填：禁止导入来源不明的知识（AGENTS.md #14）；
- `-source-type`：manual / official / interview_bank / docs；
- 政策/题库类内容必须提供 `-effective-from`，过期内容通过 `-effective-to` 标注（#16）；
- markdown 标题（首个 `# `）作为文档标题，否则用文件名；
- 导入走与 API 相同的 `knowledge.Service.CreateDocument`（分块 + embedding + 溯源元数据）。

## 10.2 检索调用方

| caller | 触发点 | 用途 |
| --- | --- | --- |
| `interview_planning` | 面试开始出题 | 基于岗位/简历知识生成问题 |
| `session_evaluation` | worker 执行评估 | 岗位名 + 全部问题作为 query，topK=5，注入评估 prompt 的参考知识 |
| `api` | `POST /api/v1/knowledge/search` | 管理/调试检索 |

## 10.3 失败与空结果策略

- 检索失败：记录 warn 日志后**退化为无参考评估**，不阻断面试/评估主链路；
- 空结果：不传参考知识，模型不得伪造来源（评估 evidence 仍需基于用户回答）；
- 每次检索计入 `rag_queries_total{caller,hit}`，延迟计入 `rag_retrieval_duration_seconds`。
