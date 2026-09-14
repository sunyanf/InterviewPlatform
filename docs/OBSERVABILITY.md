# 可观测性

## 1. 三类信号

```text
Logs
Metrics
Traces
```

---

# 2. Trace

典型：

```text
HTTP
 ↓
Interview Service
 ↓
Agent
 ↓
RAG
 ↓
LLM
 ↓
Repository
```

---

# 3. Metrics

至少：

```text
http_requests_total
http_request_duration
llm_requests_total
llm_latency
llm_tokens
llm_cost
rag_latency
rag_hit_count
agent_failures
interview_completed
```

---

# 4. LLM Usage

保存：

```text
provider
model
prompt_version
input_tokens
output_tokens
latency
cost
```

---

# 5. Request ID

所有请求必须支持：

```text
request_id
trace_id
```

便于排查：

```text
一次面试
→ 一个 Trace
→ 多次 Agent / RAG / LLM 调用
```
