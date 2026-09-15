package metrics

import "net/http"

// 进程级默认注册表：业务代码（HTTP 中间件 / LLM 装饰器 / worker / RAG）统一写入，
// /metrics 端点统一暴露。测试需要隔离时使用 New() 自建注册表。
var Default = New()

// 常用桶边界（秒）
var (
	bucketsHTTP = []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}
	bucketsLLM  = []float64{0.1, 0.25, 0.5, 1, 2, 5, 10, 30}
	bucketsTask = []float64{0.1, 1, 5, 10, 30, 60, 120, 300}
	bucketsRAG  = []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2, 5}
)

// 预定义指标（命名对齐 docs/OBSERVABILITY.md #3 与 Prometheus 命名惯例）
var (
	// HTTP
	HTTPRequests = Default.Counter("http_requests_total",
		"HTTP 请求总数", "method", "route", "status")
	HTTPDuration = Default.Histogram("http_request_duration_seconds",
		"HTTP 请求处理耗时（秒）", bucketsHTTP, "method", "route")

	// LLM（status=success|error；direction=input|output）
	LLMRequests = Default.Counter("llm_requests_total",
		"LLM 调用总数", "provider", "model", "status")
	LLMDuration = Default.Histogram("llm_duration_seconds",
		"LLM 调用耗时（秒）", bucketsLLM, "provider", "model")
	LLMTokens = Default.Counter("llm_tokens_total",
		"LLM token 消耗总数", "provider", "model", "direction")
	LLMCost = Default.Counter("llm_cost_total",
		"LLM 估算费用（美元，单价未配置时为 0）", "provider", "model")

	// 异步任务（result=success|retry|failed）
	TasksEnqueued = Default.Counter("tasks_enqueued_total",
		"异步任务入队总数（去重未入队不计）", "type")
	TaskRuns      = Default.Counter("task_runs_total", "任务执行次数", "type", "result")
	TaskDuration  = Default.Histogram("task_run_duration_seconds", "任务执行耗时（秒）", bucketsTask, "type", "result")
	TaskClaimWait = Default.Counter("task_claim_errors_total", "领取任务失败次数", "worker")

	// RAG（caller=调用方，如 interview_question_generation）
	RAGDuration = Default.Histogram("rag_retrieval_duration_seconds",
		"知识库检索耗时（秒）", bucketsRAG, "caller")
	RAGQueries = Default.Counter("rag_queries_total", "知识库检索次数", "caller", "hit")

	// Agent 与业务结果
	AgentFailures = Default.Counter("agent_failures_total",
		"Agent 编排失败次数（LLM 错误/结构化输出校验失败）", "operation")
	InterviewsCompleted = Default.Counter("interviews_completed_total",
		"面试完成（进入评估态）总数")
)

// Handler 默认注册表的抓取端点
func Handler() http.Handler { return Default.Handler() }
