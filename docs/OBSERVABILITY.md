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

## 3.1 当前实现（MVP）

零第三方依赖的进程内指标库 `pkg/metrics`（Counter/Histogram，Prometheus 文本格式），
服务启动后通过 `GET /metrics` 暴露（生产环境用网络策略限制来源）。

| 指标 | 类型 | 标签 | 含义 |
| --- | --- | --- | --- |
| `http_requests_total` | counter | method, route, status | HTTP 请求数；route 为 chi 路由模板（未匹配记 `unmatched`），避免高基数 |
| `http_request_duration_seconds` | histogram | method, route | 请求耗时 |
| `llm_requests_total` | counter | provider, model, status | LLM 调用数（success/error/stream_*） |
| `llm_duration_seconds` | histogram | provider, model | LLM 调用耗时 |
| `llm_tokens_total` | counter | provider, model, direction(input/output) | token 用量 |
| `llm_cost_total` | counter | provider, model | 估算费用（美元；单价由 LLM_PRICE_*_PER_1K 配置，未配置为 0） |
| `tasks_enqueued_total` | counter | type | 实际入队任务数（幂等去重不计） |
| `task_runs_total` | counter | type, result(success/retry/failed) | 任务执行结果 |
| `task_run_duration_seconds` | histogram | type, result | 任务执行耗时 |
| `task_claim_errors_total` | counter | worker | 领取任务失败次数 |
| `rag_retrieval_duration_seconds` | histogram | caller(api/interview_planning) | RAG 检索耗时 |
| `rag_queries_total` | counter | caller, hit(true/false) | RAG 检索次数与命中率 |
| `agent_failures_total` | counter | operation | Agent 失败次数（plan_questions/opening/analyze_answer/decide_followup/evaluate/learning_plan） |
| `interviews_completed_total` | counter | — | 面试进入完成态的次数 |

多实例部署时 Prometheus 分别抓取各实例 /metrics 聚合即可；
分片/标签级成本归因留待阶段 C 引入 OTel 时统一。

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

MVP 阶段通过 `pkg/llm.MeteredProvider` 装饰器在指标侧实时聚合上述字段
（token 来自 Provider 的 ChatResponse；流式调用只统计建连，不统计分片 token）。
逐请求 usage 落库/账单留待阶段 B 真实联调时按需补充。

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

当前实现：

- HTTP 链路：`X-Request-ID` 入站透传/自动生成，经 `pkg/requestid` 写入 ctx，
  响应头与请求日志均携带；
- 异步任务：worker 每次执行生成 `task-<task_id>-<attempts>` 作为 request_id
  写入任务 ctx，任务生命周期与评估链路日志携带该 id；
- 跨服务 trace（OpenTelemetry trace_id/span）在阶段 C 部署时引入。
