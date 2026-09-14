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
