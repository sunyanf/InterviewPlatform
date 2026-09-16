# AI 面试模拟平台 · 项目复盘与面试备战手册

> 用途：系统梳理项目从初始化至今的技术问题、功能挑战与业务难点，供面试讲述使用。
> 原则：本文所有结论均可在代码、ADR、提交历史中找到证据，不写未落地的规划。
> 配套文档：[ARCHITECTURE.md](./ARCHITECTURE.md)、[adr/](./adr/)、[EVALUATION.md](./EVALUATION.md)、[RAG.md](./RAG.md)、[SECURITY.md](./SECURITY.md)。

---

## 目录

1. [一分钟项目介绍（电梯陈述）](#1-一分钟项目介绍电梯陈述)
2. [项目全貌：功能矩阵与技术栈](#2-项目全貌功能矩阵与技术栈)
3. [技术选型依据与对比分析](#3-技术选型依据与对比分析)
4. [整体架构设计逻辑](#4-整体架构设计逻辑)
5. [核心功能模块实现思路](#5-核心功能模块实现思路)
6. [重点技术问题复盘（15 例）](#6-重点技术问题复盘15-例)
7. [业务难点与产品取舍](#7-业务难点与产品取舍)
8. [质量保障与验证体系](#8-质量保障与验证体系)
9. [关键开发历程与提交索引](#9-关键开发历程与提交索引)
10. [面试问答速记](#10-面试问答速记)
11. [已知不足与后续改进](#11-已知不足与后续改进)

---

## 1. 一分钟项目介绍（电梯陈述）

> 这是一个 **AI 驱动的模拟面试训练平台**。用户选择目标岗位（或直接粘贴招聘 JD 创建自定义岗位）、上传简历后，系统由 AI 考官围绕岗位要求和简历出题，支持文字/语音作答、实时追问对话、语音播报；面试结束后，系统基于评分细则（Rubric）和参考答案要点（Evidence）逐维度打分，生成评估报告、能力画像和学习建议。

三句话讲技术亮点：

1. **后端**：Go 单体模块化（Modular Monolith），chi + pgx + PostgreSQL/pgvector + Redis + MinIO，自研异步任务框架（租约/退避/回收）、WebSocket 实时面试通道、LLM Provider 适配层（OpenAI 兼容，可管理后台热切换）。
2. **AI 工程化**：核心原则是"**LLM 可以提意见，但不能成为系统最终事实来源**"——会话状态机、分数计算、权限、事务全部由确定性代码掌控；LLM 输出强制走「清理 → JSON Schema 解析 → 业务校验 → 归一化」管线；Prompt 版本化；RAG 知识强制带来源与有效期。
3. **可靠性**：长耗时 AI 调用全部异步化并有限重试；回答"先落库后分析"，断线不丢答案；WebSocket 支持顶替重连、快照对账与 REST 补拉；具备 Prometheus 指标、LLM 成本计量、限流、刷新令牌轮换、Playwright E2E 和黄金评估数据集回归。

---

## 2. 项目全貌：功能矩阵与技术栈

### 2.1 功能模块（对应 `backend/internal/`）

| 模块 | 核心能力 |
|---|---|
| user | 注册登录、JWT 访问令牌 + 轮换刷新令牌、角色（user/admin）、账号禁用 |
| job | 岗位分类、岗位 CRUD、支持用户粘贴 JD 创建自定义岗位 |
| resume | 简历上传（MinIO）、PDF 解析（异步任务）、岗位匹配 |
| interview | 面试会话状态机、出题、答题、追问、REST + WebSocket 双通道 |
| agent | AI 编排器：出题规划 Planner、面试官 Interviewer、追问决策 Follow-up、回答分析 Analyzer、评估 Evaluator、报告 Reporter、TTS Speech |
| knowledge (RAG) | 知识摄取 CLI、段落分块、Embedding、pgvector 向量检索、来源与有效期管理 |
| evaluation | Rubric 维度权重、证据驱动评分、确定性加权总分、黄金数据集回归 |
| report | 评估报告、能力画像、学习建议 |
| task | 通用异步任务表 + bounded worker pool（租约、指数退避、僵死回收、失败补偿钩子） |
| realtime | WebSocket 面试房：快照下发、答题帧、流式对话、TTS 帧、顶替重连、Origin 白名单 |
| audio / tts | 录音上传、ASR 转写、服务端语音合成（含预签名 URL 与缓存） |
| settings/admin | 系统设置热更新（LLM 配置热切换）、用户管理 |
| middleware | 鉴权、限流、安全响应头、请求 ID、访问日志（敏感字段脱敏）、指标 |

### 2.2 技术栈一览

| 层 | 技术 | 版本（go.mod / package.json 真实依赖） |
|---|---|---|
| 后端语言 | Go | 1.24.1 |
| HTTP 路由 | go-chi/chi v5 | 轻量、net/http 兼容、中间件组合直观 |
| 数据库驱动 | jackc/pgx v5 | 连接池、纯 Go、支持 PostgreSQL 新特性 |
| WebSocket | gorilla/websocket v1.5.3 | 事实标准，写并发约束清晰 |
| 数据库 | PostgreSQL 16 + pgvector | 关系数据 + 向量检索一体化 |
| 缓存/队列预留 | Redis 7 | compose 中部署（本地端口避让为 6380） |
| 对象存储 | MinIO（S3 兼容） | 简历、音频文件 |
| 其他 | golang-jwt/v5、google/uuid、x/crypto（bcrypt）、ledongthuc/pdf | 标准库优先，依赖极简 |
| 前端 | React 18 + TypeScript 5.5 + Vite 5 | SPA |
| 前端数据层 | @tanstack/react-query v5 | 服务端状态缓存、轮询、失效 |
| 前端路由 | react-router-dom v6 | |
| 实时通信 | 原生 WebSocket 封装（`src/ws/useInterviewSocket.ts`） | |
| 语音 | MediaRecorder 录音 + Web Speech API 识别（含上传 ASR 兜底）+ 服务端 TTS | |
| 测试 | Go testing + 表驱动/fake；Playwright E2E；自研黄金评估集 | |
| 运维 | Docker 多阶段构建、docker-compose 全栈、Prometheus、livez/readyz、CI | |

**依赖观**：后端 `go.mod` 直接依赖只有 10 个，能用标准库解决的绝不引三方库（http.Client 做重试、自写 worker pool 而不引任务框架、自写限流中间件），这对应 AGENTS.md 的 Go 规则——"不要为了架构高级制造没有实际价值的抽象"。

---

## 3. 技术选型依据与对比分析

### 3.1 为什么是 Go 而不是 Java/Python（后端）

- **场景匹配**：后端最重的负载是 WebSocket 长连接、流式转发、并发 worker——Go 的 goroutine + channel + context 取消模型天然契合，单进程即可支撑大量并发连接。
- **部署简单**：编译为单个静态二进制，多阶段 Docker 镜像小，无 JVM 内存基线负担；对个人项目运维成本极低。
- **AI 调用方不需要 Python**：LLM 能力通过 OpenAI 兼容 HTTP API 获得，Embedding 同理，业务侧只做编排，用 Go 写 HTTP 客户端和重试管线完全够用。
- 对比 Python（FastAPI）：IO 性能与并发模型、部署体积、类型安全（编译期挡住大量低级错误）Go 更优；AI 算法研究才是 Python 的主场，本项目不训练模型。
- 对比 Java（Spring Boot）：生态强但对单人开发偏重，启动慢、内存大，模块化单体用 Go 的包边界 + 接口已足够。

### 3.2 为什么是 Modular Monolith 而不是微服务（ADR-0001）

记录在 [0001-modular-monolith.md](./adr/0001-modular-monolith.md)：

- **决策背景**：单人开发、AI Coding 迭代快、早期需求变化大、核心复杂度在 AI 产品逻辑（Agent/RAG/评估）而非服务数量。
- **收益**：无 RPC/服务治理/分布式事务噪音；调试一条链贯穿；事务边界简单；AI 工具理解上下文更容易。
- **边界保障**：模块间禁止跨库直写，必须通过对方 Service 接口（见 [MODULE_BOUNDARIES.md](./architecture/MODULE_BOUNDARIES.md)）；包内按 handler/service/repository 分层，repository 不跨包引用。
- **拆分预案**：当且仅当出现明确的性能瓶颈、独立发布需求、团队边界、故障隔离需求时再拆；长期服务边界（User/Job/Resume/Interview/Agent/Knowledge/Evaluation/Report/Learning/Media）在 PROJECT_MEMORY 中已预留，等于"先画好缝，再择机切开"。

### 3.3 为什么 PostgreSQL + pgvector 一库多用（ADR-0003）

- MVP 数据规模可控（知识块、会话、简历），一个库同时承担关系存储、全文检索、向量检索，**少一个基础设施、少一套一致性方案**（知识块文本与向量天然同事务写入）。
- 对比 Milvus/Qdrant：独立向量库带来额外运维、双写一致性与备份复杂度，在数据量未到瓶颈时纯属过度设计；增长后再迁移，向量访问收敛在 knowledge repository 一层，替换成本可控。

### 3.4 LLM Provider 抽象（ADR-0002）

- 业务层（agent 包）只依赖内部 `llm.Provider` 接口（Chat、流式 ChatStream），`OpenAIProvider` 是其 OpenAI 兼容实现——OpenAI、DeepSeek、通义千问等兼容 API 均可通过配置 base_url/model/key 接入。
- 收益：厂商不锁死、可按成本/效果切换、测试可注入 fake provider、管理后台改配置即可热切换（提交 `39029ef`、`ee7a8dc`）。

### 3.5 前端：React + Vite + react-query 的理由

- Vite：开发秒级热更新，构建基于 esbuild/rollup，对比 CRA（Webpack）配置与启动速度优势明显。
- react-query：本项目服务端状态占比极高（岗位、简历、会话、异步任务轮询），用它统一管理缓存、loading/error、轮询与失效，避免手写一堆 useEffect + useState 数据同步 bug；客户端瞬时状态（录音、流式文本）才放本地 state。
- 刻意不引入 UI 组件库（antd/MUI）：视觉风格定制化（Fraunces 字体、暗色质感），组件数量不多，自建 `.btn/.card/.input/.notice` 等少量 class 即可，包体更小（gzip 后 JS 约 86KB）。

### 3.6 异步任务为什么自研而不用 Kafka

- MVP 计划 Week 11 曾列 Kafka，实际落地选择 **PostgreSQL 任务表 + 进程内 bounded worker pool**：任务量级低（简历解析、评估、报告），PG 的行锁 `FOR UPDATE SKIP LOCKED` 足以支撑多 worker 抢占；不引入 MQ 是因为没有削峰填谷、多消费者、跨服务事件的真实需求。
- 接口与处理逻辑隔离在 `task.Runner`，未来换 Redis Stream/Kafka 只需替换领取与标记实现；并通过 `TASK_ENABLED` 开关支持 API 与 worker 分进程部署（提交 `d73d2d8`）。

---

## 4. 整体架构设计逻辑

### 4.1 分层与请求链路

```text
浏览器 (React SPA)
  │  REST /api/v1/*          WebSocket /interviews/{id}/ws
  ▼                          ▼
chi Router + 中间件链（requestid → security → logger → metrics → ratelimit → auth）
  ▼
模块 Handler（参数校验、鉴权边界）
  ▼
模块 Service（业务规则、状态机、事务；AI 编排）
  ▼                         Agent 编排器（Prompt 版本化）
Repository（SQL，pgx）       │  llm.Provider 接口（重试/超时/流式）
  ▼                         ▼  knowledge 检索（pgvector + 来源过滤）
PostgreSQL / MinIO           外部大模型 API
```

AI 请求链铁律（ARCHITECTURE.md §5）：

```text
HTTP/WS → Interview Service → Agent Orchestrator → RAG → LLM
       → Schema Validation → Business Validation → Interview State → Response
```

### 4.2 四条不可动摇的架构原则（均有 ADR/宪法支撑）

1. **LLM 不拥有业务状态（ADR-0004）**：LLM 产出 `next_question / follow_up / evaluation / recommendation`，但 session 状态迁移、权限、事务、最终分数一律代码决定。例如追问：LLM 只返回 `{should_follow_up, question}`，是否创建追问题、题号多少、能否插入，由 `interview.Service` 校验会话状态与题目归属后落库。
2. **状态机唯一入口**：会话状态（INIT/READY/RUNNING/PAUSED/FINISHING/EVALUATING/COMPLETED/FAILED）只能经 repository 的 `UpdateStatus` 迁移，非法迁移返回 `INVALID_STATE_TRANSITION`，不会因为模型说"面试结束了"就置 COMPLETED。
3. **AI 输出先验证后消费**：`cleanJSON`（剥离 ```json 代码块）→ `json.Unmarshal` 进强类型 struct → 枚举/范围等业务校验（如题型只允许 technical/behavioral/project/follow_up）→ 归一化（nil slice 归一化为空数组）→ 才允许写库。禁止"LLM 字符串直接 JSON parse 后入库"。
4. **证据驱动评估**：评分输入必须是「问题 + 用户回答 + Rubric + 参考要点 ExpectedPoints + Evidence」，LLM 只给维度分与依据，**总分由代码按权重计算**（`CalculateTotalScore = Σ 维度分×权重`），Rubric 快照随评估落库以便追溯。

### 4.3 上下文、并发与 goroutine 治理

- 所有 IO（DB/HTTP/LLM/WS/对象存储）显式传递 `context.Context`；每个新增 goroutine 都能回答"谁创建、谁停止、何时退出、失败怎么办、是否泄漏"。
- WebSocket 连接采用经典 readPump/writePump 双泵：读循环绝不做重活；所有写帧只经 writePump 单 goroutine（gorilla/websocket 禁止并发写）；发送走带 64 缓冲的 channel，慢消费者（缓冲满）直接关连接避免内存堆积。

---

## 5. 核心功能模块实现思路

### 5.1 认证与安全（user + middleware）

- bcrypt 存密码；JWT 访问令牌短期有效；刷新令牌**服务端落库、一次性轮换**（刷新即作废旧令牌、签发新对），被盗可通过检测重放失效。
- 限流中间件按客户端真实 IP（`X-Forwarded-For` 可信跳数处理）区分登录接口与业务接口配额；安全中间件补常用响应头；WS 升级独立做 Origin 白名单（`ALLOWED_ORIGINS`，浏览器无法为 WS 设 Authorization 头，token 走 query，日志中间件对 token 脱敏）。
- 前端刷新令牌持久化到 localStorage 并在 401 时静默刷新、重放原请求（提交 `6546c2f`）。

### 5.2 岗位与简历

- 岗位与分类分表；岗位列表分页、按分类过滤；**列表服务层保证空结果返回 `[]` 而非 `null`**（这是后述前端白屏事故的根因防线）。
- 自定义岗位：任意登录用户可粘贴 JD 创建岗位（标题/分类/职责/任职要求/技能），source 标记 `custom`，创建后即进入面试准备页配置题型/题量/时长。
- 简历：文件落 MinIO、元数据落库；PDF 文本解析作为异步任务，前端凭 task_id 轮询；解析结果驱动岗位匹配与出题个性化。

### 5.3 面试会话状态机与出题

- 创建会话时校验岗位与简历，配置（题型、题量、时长）以 JSON 快照存储；启动时调用 Planner：输入岗位技能/要求 + 简历技能 + 题型题量 + RAG 召回知识，输出结构化题目（题干、类型、难度、ExpectedPoints）。
- 答题链路（`SubmitAnswer`）设计为**三段式：先落库 → 再尽力分析 → 再决策追问**。回答唯一约束防重复答题（409 ALREADY_ANSWERED）；普通题才允许追问，防止追问链无限延长。
- 双通道：REST 提供完整资源（快照、开场白话术），WS 承担面试房实时交互。

### 5.4 Agent 多角色拆分

不做"超级 Agent"，按职责拆为 6 个 prompt 角色（[PROMPT_REGISTRY.md](./agent/PROMPT_REGISTRY.md) 管理版本）：Planner 出题、Interviewer 对话/开场白、Follow-up 追问决策、Analyzer 回答分析（claims/对错点/遗漏点/知识缺口）、Evaluator 维度评分、Reporter 报告与学习建议。每个角色独立 prompt、独立温度、独立 struct 契约与测试，改一个角色不影响其他角色。

### 5.5 RAG 知识库

- 摄取 CLI：校验来源 → 分块 → Embedding → 与 source/source_type/effective_from/effective_to/version 一起入库。
- 分块策略（`chunker.go`）：目标 500 字、上限 800 字；优先按段落聚合，超长段落按句子边界切分，保证语义完整不跨主题。
- 检索：pgvector 余弦相似度，评估/出题时检索并按有效期、来源类型过滤；**无结果时优雅降级为纯模型作答**，不允许把来源不明内容标成"官方标准/真实题库"。

### 5.6 评估与报告

- Rubric 默认四维：技术正确性 0.30、技术深度 0.25、逻辑思维 0.25、表达 0.20；`ValidateRubric` 强制权重和为 1。
- 评估为异步任务，幂等 Upsert（可安全重试）；报告模块聚合维度分、Evidence、能力画像 profile 与学习建议，支持历史对比。
- 自研黄金数据集 + evaluator 回归 runner（`evalrun` 包，提交 `4332ca3`）：prompt/模型变更后跑一批标准问答，防止"改 prompt 修好 A 场景却劣化 B 场景"。

### 5.7 异步任务框架（task 包）——后端工程含量最高的模块之一

- **模型**：tasks 表记录类型、payload JSON、attempts、max_attempts、状态、租约 owner/到期时间、last_error。
- **领取**：worker 用 `FOR UPDATE SKIP LOCKED` 抢占，多 worker 互不阻塞。
- **可靠性四件套**：
  1. **租约（lease）**：领取即加租约，worker 崩溃后任务不会永久锁死；
  2. **reaper**：独立 goroutine 定期回收超时任务重新入队；
  3. **指数退避**：失败后 10s 起步、翻倍、上限 2min；超过最大尝试置 failed；
  4. **失败补偿钩子**：任务终态失败时用独立 ctx 回写业务状态（如把面试评估标记失败），避免业务永久挂在 EVALUATING。
- **优雅关闭**：Shutdown 停止领新任务，有界等待在途任务；在途任务 ctx **故意不继承 runner 取消信号**（`context.Background()` + 租约超时），关闭瞬间任务可继续做完，没做完的锁交给 reaper。
- 处理器必须业务幂等（评估/报告均为 Upsert），这是"至少一次"投递模型下的硬性约定。

### 5.8 实时面试房（realtime 包）

- 连接建立：query token 鉴权 + 会话归属校验（403/404 在升级前以普通 HTTP 返回）→ 升级 → Hub 注册 → **立即下发全量 snapshot**（重连恢复 UI 不依赖服务端事件持久化）。
- 同会话单连接语义：`Hub.replace` 新连接顶替半开旧连接（自定义 close code 4000），`clear` 用指针比较防止旧连接延迟退出误删新连接；HubManager 引用计数回收 Hub。
- 消息类型：ping/pong 保活、answer（结构化答题）、chat（自由对话）、answer_saved/analysis/analysis_skipped/follow_up、chat_delta（流式 token）/chat_speech/chat_done/error。
- 在途控制：`answerInFlight`、`chatInFlight` 两个原子布尔保证每类耗时消息同时只有一轮，防止帧流交错和重复提交。
- 流式对话 LLM 生命周期与连接绑定：监听 `client.closed`，连接断开立即 cancel LLM ctx，避免白烧 token。

### 5.9 语音链路

- **输入双方案**：首选浏览器 Web Speech API（零成本、实时 interim 文本），不可用时降级为 MediaRecorder 录音上传服务端 ASR。
- **输出**：服务端 TTS 合成开场白/问题/对话回复，音频落 MinIO，返回预签名下载 URL（带过期时间），有缓存标记。
- 前端录音组件同时管理 MediaRecorder 生命周期、SR 识别会话、最大时长定时器、watchdog、自动重启计数、卸载清理，是前端状态最复杂的组件。

### 5.10 可观测与运维

- `/livez`（进程存活）与 `/readyz`（依赖就绪）分离，供编排层区分重启与摘流。
- Prometheus `/metrics`：HTTP  RED 指标、任务运行数/时长（按类型与结果）、LLM 调用失败计数与 **token 用量计量（成本可核算）**。
- 结构化 slog 日志带 request_id；异步任务自行生成 `task-{id}-{attempt}` 形式 request_id 贯穿日志。
- 多阶段 Dockerfile 构建小镜像；compose 一键起全栈 + Prometheus；数据库迁移独立为 `cmd/migrate`，与应用启动解耦。

---

## 6. 重点技术问题复盘（15 例）

> 每例按「表现与场景 → 分析过程 → 解决方案与步骤 → 效果与经验」组织，可直接作为面试 STAR 素材。

### 问题 1：WebSocket 读循环被 LLM 调用阻塞，60 秒后连接被服务端断开

**对应提交**：`1d274c5`、`1d274c5` 前的 TTS 联调期；代码见 [realtime/handler.go](../backend/internal/realtime/handler.go)。

1. **表现与场景**：用户在面试房提交回答后，页面卡顿约几十秒；期间页面任何操作无响应，约 60 秒后 WS 断开重连。网络偶发慢、模型高峰期复现明显。
2. **分析过程**：读循环（`conn.ReadMessage()` 所在 goroutine）里同步执行了 `SubmitAnswer`，其中包含回答分析 LLM 调用（可达数十秒）。读循环被占住期间无法消费客户端 ping/pong，服务端 `pongWait=60s` 读空闲超时到期即关闭连接。根因是**把慢 IO 放在了连接健康心跳的关键路径上**。
3. **解决方案**：
   - answer/chat 一律异步派发（`dispatchAnswer/dispatchChat` 起 goroutine），读循环只负责收帧、校验、派发；
   - 引入 `answerInFlight/chatInFlight` 原子标志做并发闸：同类消息已有在途任务直接回 `ANSWER_IN_PROGRESS`，既防重复提交又防帧交错；
   - 所有出站帧统一走 send channel → writePump，遵守 gorilla 单写者约束；
   - 流式 chat 用独立 streamCtx，连接关闭即 cancel LLM。
4. **效果与经验**：答题期间心跳正常、连接稳定；慢 LLM 不再影响连接健康。**经验：长连接协议必须把"协议层循环"和"业务慢操作"隔离，心跳路径上禁止任何不可控耗时。**

### 问题 2：断线/被顶号导致已提交回答丢失、回答永远没有分析

**对应提交**：`f20a107`；代码见 [interview/service.go](../backend/internal/interview/service.go) `SubmitAnswer`。

1. **表现与场景**：用户答完一题瞬间网络抖动，或同账号在另一个标签页打开面试房（旧连接被顶替 close 4000），重连后发现回答没保存或没有 AI 分析。
2. **分析过程**：提交链路沿用 WS 连接的 ctx，连接一断 `ctx.Done()`，正在执行的"落库/分析"被整体取消——但用户表达意图已经完成，取消等于丢数据。另外原顺序是分析和落库耦合，分析失败会影响回答保存。
3. **解决方案**：
   - 重排为「**先 CreateAnswer 落库，再 AnalyzeAnswer，再 DecideFollowUp**」，落库是第一道确定性事实；
   - 三段全部使用 `context.WithoutCancel(ctx)` 脱离连接生命周期，各配独立超时（提交总超时、分析超时、追问超时）兜底防 goroutine 泄漏；
   - 分析/追问改为**尽力而为**：失败只打点告警（`metrics.AgentFailures`），不影响回答提交结果；
   - 分析缺失时 WS 显式下发 `analysis_skipped` 事件，而不是让前端无限等待。
4. **效果与经验**：断线重连后回答必在，分析可能延迟但不缺失主数据；前端不再永久转圈。**经验：用户已确认提交的意图数据要先持久化，衍生型 AI 加工降级为尽力而为；连接 ctx ≠ 业务 ctx。**

### 问题 3：WS 重连后答题区状态错乱、气泡重复或永久"分析中"

**对应提交**：`a1d7112`；代码见 [useInterviewSocket.ts](../frontend/src/ws/useInterviewSocket.ts)。

1. **表现与场景**：弱网频繁断线重连时，同一条回答可能出现两个气泡；或分析其实已完成，UI 一直停在"分析中"；AnalysisView 偶发空指针白屏。
2. **分析过程**：服务端重连只推 snapshot 不补事件帧，断线期间完成的 answer_saved/analysis 事件永久丢失；而前端提交时先插入了本地占位事件，重连后没有任何机制把占位事件与服务端事实对账。
3. **解决方案**（前端状态恢复三件套）：
   - **snapshot 对账**：`reconcileFromSession` 用 snapshot 中 `q.answered && q.answer` 的题目匹配本地 answerEvents（按 questionId 找占位并复用 key），杜绝重复气泡；
   - **REST 补拉**：若对账后仍有"已提交但 snapshot 里没答案/分析"的悬置事件（分析脱离 ctx 最多还要跑数十秒），用一次性的轮询定时器（`pollsArmed` 防频繁重连下重复排期）补拉会话接口直到事实补齐；
   - **busy 三阶段 + 终态信号**：提交中/保存中/分析中状态显式区分，`analysis_skipped` 也是终态，任何终态都能结束转圈；AnalysisView 对空数据做防御。
4. **效果与经验**：地铁弱网、杀进程重开、多标签顶号后状态均能收敛一致。**经验：实时系统"重连恢复"要以服务端全量事实为基线对账本地乐观状态，事件丢失是常态而非异常；每个"进行中"状态必须有可达终态。**

### 问题 4：浏览器语音识别（Web Speech API）文本抖动、重复与自动断听

**对应提交**：`9e0c2a1`、`f7a9360`、`cee7fb6`、`e8f16b5`；代码见 [VoiceRecorder.tsx](../frontend/src/interview/VoiceRecorder.tsx)。

1. **表现与场景**：语音答题时识别文本频繁回退/跳动；Chrome 的 SR 在静默数秒后自动触发 `onend` 停止识别，用户还在说却不再转写；偶发同一段文字重复追加。
2. **分析过程**：Web Speech API 的 interim result 会被后续识别**反复修正甚至回退**，直接逐帧渲染就抖动；`resultIndex` 标识本次回调的起始结果序号，不按它区分新旧结果就会重复拼接；continuous 模式下静默自动结束是规范行为，需要显式自动重启并限制次数防止死循环。
3. **解决方案**：
   - 状态拆分为 `committed`（final，绝不回退）+ `interim`（临时预览，可随时覆盖）两段渲染，保证用户看到的已定稿文字稳定；
   - 按 `resultIndex` 增量消费结果列表去重；
   - `onend` 且非用户主动停止时静默自动 restart，用 `autoRestartsRef` 计数封顶；
   - 加最大录音时长 timer 与 watchdog 双重收尾，`finishedRef` 保证一次录音只收尾一次；组件卸载清理全部定时器与 MediaStream；
   - SR 不可用/失败时降级 MediaRecorder 录音上传 ASR。
4. **效果与经验**：识别预览稳定、静默不断听、无重复段，浏览器不支持时仍可录音答题。**经验：使用非规范化的浏览器能力（Web Speech API 仅在 WebKit/Chrome 完整可用）必须读懂其事件语义并准备服务端兜底；乐观 UI 要区分"已定稿"和"暂定稿"。**

### 问题 5：LLM 返回内容不可靠——代码块包裹 JSON、空 choices、网络抖动

**对应提交**：`f8a5c90`；代码见 [pkg/llm/openai.go](../backend/pkg/llm/openai.go)、[agent/agent.go](../backend/internal/agent/agent.go)。

1. **表现与场景**：不同模型/温度下偶发返回 ` ```json\n{...}\n``` ` 包裹导致解析失败；高负载期 429/5xx 与连接重置直接让出题/评估失败；部分兼容厂商在异常时返回空 choices。
2. **分析过程**：把 LLM 当成"严格 JSON 服务"是错误假设；同时所有错误无差别重试会放大对 400（请求本身错）的无效调用。
3. **解决方案**：
   - `cleanJSON` 统一剥离 markdown 围栏与空白，再进强类型 `json.Unmarshal`，解析失败作为显式错误进入调用方降级分支；
   - Provider 层有限重试（2 次）：**仅对网络层错误（status=0）与 429/5xx 重试**，4xx 立即失败；指数退避且尊重 ctx 取消；
   - 空 choices 显式报错；非流式 client 60s 整体超时，流式 client 不设超时、完全由 ctx 控制生命周期；
   - 补齐 AI 异常路径测试：空输出、字段缺失、类型错误、超时、重试次数断言（agent_test、openai_test）。
4. **效果与经验**：模型输出差异被吸收在适配层，业务编排只面对干净 struct 或明确 error。**经验：LLM 是"最不可靠的外部依赖"，要像对待第三方支付接口一样做输入容错、错误分级、重试边界与超时治理。**

### 问题 6：RAG 知识来源不可追踪、检索空结果引发风险

**对应提交**：`8500726`；代码见 [knowledge/service.go](../backend/internal/knowledge/service.go)、[chunker.go](../backend/internal/knowledge/chunker.go)。

1. **表现与场景**：早期摄取接口不强制来源字段，存在把来源不明的资料当"官方标准"喂给模型的风险；检索层在无召回时返回 nil，上层若直接取切片可能异常或把空上下文误当"无此知识"。
2. **分析过程**：面试/考试类知识（尤其公务员政策）时效性强，过期或来源不明信息一旦进入 prompt，模型会自信地引用，风险外溢到最终评分。
3. **解决方案**：
   - 摄取强校验：source 必填（空则 `INVALID_SOURCE` 拒绝），source_type 限定 manual/official/interview_bank/docs 枚举，effective_from/effective_to 必须合法 RFC3339；
   - 分块保证语义完整（段落聚合 500 字、句子边界 800 字封顶）；
   - 检索服务层对 nil 结果归一化空切片，上层明确"无召回=不启用 RAG 语境"，prompt 中要求模型只依据提供材料作答；
   - 知识行带 collected_at/version 落库，支持按有效期过滤。
4. **效果与经验**：每条可被模型引用的知识都可回溯来源与时效。**经验：RAG 的难点不在向量检索本身，而在数据治理——来源、时效、分块质量与无结果降级。**

### 问题 7：简历解析/评估/报告等长耗时 AI 任务同步阻塞请求

**对应提交**：`ce58926`、`6f43e73`、`7afffab`、`d73d2d8`。

1. **表现与场景**：上传简历点解析、面试结束发起评估时，HTTP 请求要等几十秒，网关/浏览器易超时；服务重启时任务半途而废且无法恢复；前端只能干等。
2. **分析过程**：这些任务天然是异步的，且需要可重试、可观测、可水平扩 worker；同步接口无法满足。
3. **解决方案**：新建 tasks 表与 Runner（§5.7）：API 只入队并返回 `202 + task_id`；worker 租约抢占执行，失败指数退避、超限 failed 并触发业务补偿钩子；前端 react-query 凭 task_id 轮询状态；`TASK_ENABLED` 支持 API/worker 拆分部署。
4. **效果与经验**：接口秒回，重启不丢任务（租约回收重跑），任务成功率/耗时/失败原因全部可观测。**经验：耗时外部调用遵循"快速受理 + 后台执行 + 状态可查 + 幂等重试"四件套；至少一次投递下幂等键/Upsert 是业务方的必修课。**

### 问题 8：安全加固——令牌重放、暴力登录、跨站 WS、敏感信息入日志

**对应提交**：`b925943`、`538ba90`。

1. **表现与场景**：初版刷新令牌为长期不轮换票据，泄露后可无限使用；登录接口无限流可被撞库；WS 无 Origin 校验存在跨站握手风险；token 走 query 可能被访问日志/Referer 泄露。
2. **分析过程**：按威胁建模逐项核对 OWASP 常见面：认证、暴力破解、CSWSH、信息泄露。
3. **解决方案**：刷新令牌服务端存储且**一次性轮换**（重用可被发现）；限流中间件对登录与业务 API 分别配额并正确解析客户端 IP；WS 升级强制 `ALLOWED_ORIGINS` 白名单；日志中间件对 token/密码类字段脱敏；安全响应头中间件；密码仅 bcrypt 哈希。
4. **效果与经验**：安全中间件均有独立单测（security_test、ratelimit 逻辑随用户服务测试覆盖）。**经验：安全要在基础设施期就进中间件链而不是事后补；长连接的浏览器鉴权特殊性（query token）要有配套脱敏与 Origin 防护。**

### 问题 9：前端 access token 过期导致用户频繁被踢回登录页

**对应提交**：`6546c2f`。

1. **表现与场景**：面试进行到一半 token 过期，下一个请求 401，页面跳登录，体验中断。
2. **分析过程**：access token 短命是安全刚需，矛盾要靠"静默刷新 + 请求重放"解决，且要防止多个并发请求同时 401 触发多次刷新。
3. **解决方案**：token 对（access/refresh）存 localStorage；API 客户端封装统一响应层：遇 401 用单飞（singleton promise）刷新令牌后重放原请求；刷新失败才清空令牌跳登录。
4. **效果与经验**：正常使用几乎感知不到令牌过期。**经验：鉴权体验设计的关键是"刷新流程并发去重"，否则一次过期会放大成 N 次刷新。**

### 问题 10：首页"已答"题数全部显示 0、岗位名为空

**对应提交**：`d0ab7de`（本项目复盘当期修复）。

1. **表现与场景**：首页面试列表每条都显示"已答 0/N"，点进详情页数字却正确。
2. **分析过程**：详情接口有 `COUNT(*) FROM interview_answers` 统计和岗位标题查询，而**列表接口的 SQL 只查了 session 主表**——典型的"详情接口字段齐全、列表投影漏字段"问题；前端严格依赖 `answered_count` 渲染，于是全 0。
3. **解决方案**：列表 SQL 改为 LEFT JOIN jobs 取标题、相关子查询统计回答数（一条 SQL 完成，避免 N+1），Scan 对齐新增两列；前端无需改动。
4. **效果与经验**：接口实测 6 场会话返回 0/2/3/4 真实值，UI 与接口一致。**经验：列表 DTO 的字段投影要有契约核对（同一资源在列表/详情的字段差异要显式设计）；修显示问题先抓真实接口响应，区分数据源问题还是渲染问题。**

### 问题 11：空岗位分类返回 `list:null`，前端取 `.length` 整页白屏

**对应提交**：`661e984`、`4803c75`。

1. **表现与场景**：岗位页点到没有岗位的分类标签（如公务员类），整页黑屏/白屏崩溃，控制台 `Cannot read properties of null`。
2. **分析过程**：Go 的 slice 零值是 nil，`json.Marshal(nilSlice)` 序列化为 `null`；前端直接 `jobList.list.length` 对 null 取属性抛异常；skills/requirements 同理会崩卡片。这是**跨语言序列化契约不对齐**：Go 认为 nil 与空切片"差不多"，JS 认为 null 与 [] 完全不同。
3. **解决方案**（双端防线）：
   - 后端：Service 层列表查询 nil 时归一化为 `[]Job{}`，从源头保证集合字段永不为 null；
   - 前端：`list?.length ?? 0`、`skills ?? []` 等可选链 + 空值合并全面兜底；
   - 空态补引导 CTA（自定义岗位），把错误体验变成功能入口。
4. **效果与经验**：空分类显示友好空态不再崩溃。**经验：API 集合类型的空值约定要全链路统一（推荐后端永远返 []）；前端对外部数据默认不可信；空态页要承担引导职责。**

### 问题 12：多标签页打开同一面试房引发"顶号风暴"

**涉及代码**：`Hub.replace/clear/release` + close code 4000。

1. **表现与场景**：测试时浏览器残留多个面试标签页，每个标签都建立 WS，旧连接被新连接顶替后旧页面重连又顶掉新连接，循环抢连接。
2. **分析过程**：同会话单连接是产品语义（一个考生一场面试一个实时通道），但"半开连接"（客户端已断、服务端读循环尚未感知失败）必须由新连接显式顶替；同时旧连接的延迟退出不能误清新连接。
3. **解决方案**：Hub 用 mutex 保护当前 client 指针，replace 返回旧连接并以 4000 关闭；clear 做指针相等判断；HubManager 在连接彻底退出（等待 writePump 关闭）后双重检查再回收 Hub；前端被顶后给明确提示而非无脑立刻重连。
4. **效果与经验**：同账号多标签冲突收敛为确定的"新连接生效"。**经验：长连接做单端互踢要处理好"旧连接延迟死亡"的竞态（指针比较 + 引用计数），关闭流程要有明确 close code 供客户端区分原因。**

### 问题 13：本地基础设施冲突——DockerHub 拉取失败、Redis 端口被占

**对应提交**：`38342f3`、`e575ae4`。

1. **表现与场景**：`docker-compose up` 拉 MinIO 官方镜像超时失败；本机已有 Redis 占用 6379，compose 启动端口冲突；CI 与本地 Go 版本不一致导致构建差异。
2. **分析与解决**：MinIO 换用 `quay.io/minio/minio` 镜像源；compose 中 Redis 映射 6380 避让本机服务；统一 Go 1.24.1（CI 对齐）；依赖与配置全部环境变量化（.env + godotenv）。
3. **效果与经验**：新机器按 LOCAL_SETUP 一条命令可起全栈。**经验：环境问题要有文档化的标准解法；端口设计要预判开发者机器已有服务。**

### 问题 14：LLM 配置变更需要改代码重启、厂商切换风险高

**对应提交**：`39029ef`、`ee7a8dc`。

1. **表现与场景**：初期模型配置写在 env，换模型/换 key 要重新发版；生产环境一旦某厂商限流或故障无法快速切走。
2. **分析与解决**：Provider 抽象只解决"能换"，管理后台 settings + 热更新解决"随时换"：LLM 配置入库（api_key 密码框展示），更新后 Provider 工厂原子替换（热切换），限流/worker 等不适合热更的参数明确标注"重启生效"；后台每一项配置配中文名与说明，避免误操作。
3. **效果与经验**：模型切换分钟级完成且无需发版。**经验：把"会变的东西"（模型、key、限流值）与"不变的东西"（代码逻辑）分离；热更新要明确边界，不能热更的要在 UI 诚实标注。**

### 问题 15：AI 评分主观性争议——如何让分数可信、可回归

**对应提交**：`2ab4c75`、`4332ca3`；代码见 [evaluation/rubric.go](../backend/internal/evaluation/rubric.go)、`evalrun` 包。

1. **表现与场景**：若让 LLM 直接输出"综合 78 分"，同一回答多次评分可能波动，用户无法理解分从何来，也无法验证改动 prompt 是否让评分变差。
2. **分析过程**：非确定性模型不能直接拥有最终分数（ADR-0004 与宪法 §15）。
3. **解决方案**：LLM 仅按 Rubric 维度（正确性/深度/逻辑/表达）输出**维度分 + 引用证据**；总分由代码加权求和并四舍五入；Rubric 与评估结果一同快照落库；建立黄金问答数据集与 evaluator 回归 runner，模型/prompt 变更必须跑回归对比。
4. **效果与经验**：分数可解释、可复算、变更可回归。**经验：AI 系统的信任来自"人类可审计的证据链 + 确定性代码兜底最终结论"，而不是模型的自我声明。**

---

## 7. 业务难点与产品取舍

1. **确定性与非确定性的边界划分**：这是全项目最核心的业务难点。状态、计时、权限、事务、总分、配额必须 100% 可预测；理解、出题、分析、建议允许概率性。工程上体现为：LLM 永远不直接写状态表，所有 AI 结论先过校验管线。
2. **AI 的"尽力而为"降级体系**：模型会超时、限流、返回脏数据。设计上把用户价值分层——回答内容必须保住（先落库）、分析可以稍后补（异步 + skipped 事件）、追问可以没有、RAG 可以无召回、TTS 失败不影响文字。任何 AI 子能力故障都不应拖垮主流程。
3. **评分可信度**：用 Rubric + ExpectedPoints + Evidence 约束模型自由发挥；维度分明细展示给用户；代码算总分；黄金数据集回归。
4. **知识时效性与合规**：公务员/考试方向的政策法规强时效，知识必须有 source/effective_from/effective_to/version，过期材料不能默认当现行标准；不允许标记来源不明的"真题"。
5. **Prompt 即代码**：Prompt 纳入版本管理（PROMPT_REGISTRY），AI 输出结构自带 `prompt_version`、`model` 字段落库，任何一次评分都能追溯到"哪个模型、哪版 prompt"。
6. **自定义岗位（用户自助 HC/JD）**：真实招聘平台岗位维护成本高，而练习场景的核心是"围绕我要面的 JD 练"。取舍为：任意登录用户可粘贴 JD 自建岗位（source=custom），不与真实招聘 HC/流程打通——用最小功能满足核心诉求，明确不做招聘 ATS。
7. **语音体验的投入取舍**：服务端 ASR/TTS 成本高，故语音输入优先用免费的浏览器原生识别、录音上传作为兜底；语音输出按需合成并缓存，而不是每题无条件合成。
8. **隐私最小化**：简历/音频属敏感数据，日志禁止打印完整简历、音频、令牌；对象存储用预签名 URL 限时访问。

---

## 8. 质量保障与验证体系

| 层次 | 手段 | 具体内容 |
|---|---|---|
| 单元测试 | Go 表驱动测试 + 接口 fake | agent（脏 JSON/空字段/超时）、llm（重试/4xx 不重试/5xx 重试）、task（抢占/退义/回收）、middleware（安全头/Origin）、rubric（权重校验/加权）、knowledge、resume、interview、report、tts 等均有 `*_test.go` |
| AI 专项测试 | 异常输出矩阵 | 模型空输出、字段缺失、类型不符、枚举越界、RAG 无结果、LLM 超时与重试次数 |
| AI 回归 | 黄金数据集 | `evalrun` 跑标准问答集，评估器变更前后可对比（提交 `4332ca3`） |
| 集成/E2E | Playwright happy path | 登录→选岗→上传简历→面试→报告主链路，CI 中运行且失败重试有报告（`adce5e0`） |
| 质量门禁 | gofmt / go vet / tsc / vite build | 每次修改后全量执行 |
| 可观测 | Prometheus + 结构化日志 | RED 指标、任务指标、LLM 失败与 token 成本、request_id 全链路 |
| 韧性验证 | 压测与故障演练 | 全栈 compose + Prometheus + load test（`6e58637`）；手工演练断网、杀 worker、多标签顶号、模型超时 |
| 数据迁移 | 独立 migrate 命令 | 迁移与应用启动解耦，破坏性变更需人工确认 |

**测试观**：AI 功能测的不是"模型聪不聪明"，而是**系统面对模型一切异常输出时是否安全**——这正是单测中大量构造脏数据的原因。

---

## 9. 关键开发历程与提交索引

按 MVP 计划约 12 周推进，实际提交历史（50 commits）分四个阶段：

**阶段一：垂直切片打底（Week 1–7，提交 `89c0ddf`→`0d8b549`）**
- Week1 骨架/Auth/DB/HTTP Server；踩坑 MinIO 镜像源、Redis 端口（`38342f3`）
- Week2 岗位+简历；Week3 面试核心与状态机；Week4 Agent 多角色
- Week5 RAG + 风险加固（`beb5f38`、`8500726`）；Week6 评估 Rubric/Evidence；Week7 报告
- 穿插两次关键加固：LLM 调用加固与异常测试（`f8a5c90`）

**阶段二：实时化与语音（Week 8–9，`db6a72b`→`e8f16b5`）**
- 音频上传/ASR（`e279d9a`）、WebSocket 实时面试（`a102502`）、TTS（`0476495`）
- 浏览器真实 SR + 上传兜底（`f7a9360`）、浏览器 TTS 适配（`cee7fb6`）、语音控件与重连闪烁（`e8f16b5`）
- readPump 解耦 LLM（`1d274c5`）、断线不丢答案（`f20a107`）、SR 稳定化（`9e0c2a1`）、重连对账（`a1d7112`）

**阶段三：工程化补强（`50fa39f`→`3cedd04`）**
- 健康检查、WS Origin、多阶段镜像、独立迁移命令
- 异步任务体系四连（任务表→接业务→前端轮询→API/worker 分离）
- 安全加固（刷新令牌轮换/限流）、前端静默刷新、Prometheus 与成本计量、RAG 摄取 CLI、E2E
- 生产镜像/全栈 compose/压测、管理后台与 LLM 热切换

**阶段四：产品闭环打磨（`1293624`→`6716e85`）**
- 自由对话语音输入、停止朗读大按钮、首页统计修复、空分类崩溃修复、自定义岗位入口、后台全面中文化

---

## 10. 面试问答速记

### 10.1 三个值得主动讲的 STAR 故事

**故事 A：一次 WebSocket 答题数据丢失的系统性修复（最能体现全栈深度）**
- S：实时面试房在弱网/多标签下回答丢失、分析缺失、重连状态错乱。
- T：保证"用户已提交的回答绝不丢、断线后 UI 与服务端最终一致"。
- A：三层修复——① 读循环与 LLM 解耦保证连接健康；② 回答先落库、分析与追问脱离连接 ctx 并独立超时、失败显式 skipped；③ 前端以 snapshot 全量事实对账本地乐观事件，悬置事件 REST 补拉且只排一次轮询。
- R：断网、杀进程、顶号场景数据零丢失，状态自动收敛；沉淀出"连接 ctx ≠ 业务 ctx""进行中状态必有终态"两条规范。

**故事 B：把不可靠的 LLM 变成可依赖的系统组件**
- S：模型输出脏 JSON、空 choices、429/超时频发，直接威胁出题评分可用性。
- T：在不换模型的前提下让 AI 链路可控、可测、可信。
- A：Provider 适配层错误分级重试与超时；输出清清洗 + Schema + 业务校验；LLM 不碰状态/总分；Prompt 版本化与黄金数据集回归；管理后台热切换厂商。
- R：模型故障降级为局部能力缺失而非主流程失败；模型与 prompt 变更可回归审计。

**故事 C：零 MQ 的可靠异步任务设计**
- S：简历解析/评估等数十秒 AI 任务同步阻塞且重启即丢。
- T：单人项目以最小基础设施获得可重试、可恢复、可扩缩的异步能力。
- A：PG 任务表 + SKIP LOCKED 抢占 + 租约 + reaper + 指数退避 + 幂等 Upsert + 失败补偿 + 有界优雅关闭，API/worker 可分进程部署。
- R：接口秒回、崩溃恢复、指标齐全；未来可平滑替换为 MQ。

### 10.2 高频追问题库（要点版）

- **Q：状态机怎么保证不被 LLM 越权？** A：状态迁移只在 repository 单一入口做合法性校验；LLM 输出的是建议 struct，由 service 校验会话状态/题目归属后才落库；非法迁移 400。
- **Q：WS 鉴权怎么做？token 在 query 安全吗？** A：浏览器 WS 不能自定义头，token 走 query 并走 wss（TLS），服务端升级前校验 JWT 与资源归属，配合 Origin 白名单防 CSWSH，日志对 query token 脱敏；短时效 + 刷新机制降低泄露窗口。
- **Q：服务端怎么知道客户端死了？** A：pongWait 60s 读空闲超时 + ping/pong；客户端异常时新连接顶替（4000）；写缓冲满判定慢消费者主动关。
- **Q：LLM 超时怎么处理？** A：非流式 60s client 超时 + ctx 截止；流式不设整体超时、由 ctx 控制；业务侧分析/追问各自独立超时且失败降级；Provider 仅对网络错/429/5xx 重试两次。
- **Q：重复提交/幂等怎么做？** A：答题有唯一约束（同题重复答 409）；WS 在途原子闸拦截重复帧；异步任务"至少一次"投递，业务处理全部 Upsert。
- **Q：worker 正在跑任务时发布/重启怎么办？** A：停止领新任务、有界等待在途任务；任务 ctx 不随关闭取消，可做到租约上限；没做完的由 reaper 回收重跑，配合幂等不会产生重复业务数据。
- **Q：pgvector 检索怎么保证质量？** A：段落聚合 + 句子边界切分（500/800 字）保证语义完整；带来源类型与有效期过滤；无召回显式降级；未来数据量增长可整体替换为专用向量库。
- **Q：为什么不用微服务/Kafka/Redis 队列？** A：当前负载与团队规模下收益为负；已用模块边界与接口预留拆分缝、用 PG 行锁实现队列语义，等真实瓶颈（指标可证）出现再迁移，避免过早复杂度。
- **Q：前端怎么管理这么多异步状态？** A：服务端状态归 react-query（缓存/轮询/失效），实时状态归 WS hook 的单一 reducer 式 state（session/bubbles/streaming/answerEvents），乐观事件带稳定 key 供对账；录音等设备状态用 ref 管理命令式生命周期。
- **Q：怎么评估 prompt 改动的好坏？** A：Prompt 版本号随结果落库；黄金数据集 + evaluator 回归 runner 对比维度分漂移；再结合线上失败打点与人工抽查。
- **Q：项目里最有挑战的 bug？** A：用故事 A（WS 数据丢失三层修复），强调"先证据定位（真实接口/日志/复现）再分层修、每修一层配验证"。
- **Q：如果让你重构，最先改什么？** A：① 事件持久化（把 WS 关键事件落库，重连补事件而非全量对账）；② 评估链路引入消息队列支持批量面试；③ 知识检索加混合检索（关键词+向量）与 rerank；④ 前端组件的状态机化（录音/面试房）。
- **Q：怎么控制 LLM 成本？** A：token 用量打点入 Prometheus 可按场景核算；流式中断即取消；TTS 按需合成+缓存；后台可配置每千 token 价格参数用于成本统计。
- **Q：SQL 注入/越权访问怎么防？** A：pgx 参数化查询；所有资源访问带 userID 归属校验（getOwnedSession）；管理接口 RequireAdmin；行级数据在 service 层强制过滤。

### 10.3 可量化的项目事实（讲述更有说服力）

- 后端 10 个直接依赖、20+ 内部模块包；前端 gzip JS ≈ 86KB；50 个规范 conventional commits。
- 实时链路：心跳 50s/读超时 60s、发送缓冲 64、单条对话上限 2000 字、WS 读消息上限 1MB。
- 异步任务：默认 2 worker、轮询 2s、租约 5min、退避 10s→2min 封顶、关闭等待 20s、reaper 间隔≥30s。
- RAG：分块目标 500 字/上限 800 字；来源类型 4 种枚举。
- LLM：默认单次调用 30s 超时、非流式 HTTP 60s、重试 2 次、退避 200ms 起步。
- 评估：4 维权重 0.30/0.25/0.25/0.20，总分代码加权并保留两位小数。

> 注：以上数字均可在代码常量中直接指出，面试官追问时可现场翻代码佐证，这比抽象描述更有说服力。

---

## 11. 已知不足与后续改进

诚实面对，面试中主动提及反而是加分项：

1. **WS 事件未持久化**：当前重连恢复依赖全量 snapshot + 前端对账，极端情况下断线期间的"过程性信号"（如流式回复中断点）无法精确续传；后续应把关键事件入库并支持按序号补拉。
2. **无多租户与组织概念**：目前岗位/知识为全局或个人维度，没有企业招聘方视角的 HC 批量面试、面试官协作。
3. **RAG 检索较基础**：目前以向量相似度为主，缺少 BM25/全文混合检索与 rerank；公务员等题库数据量尚小。
4. **评估维度固定**：Rubric 虽已快照化并预留按岗位差异化，但尚未开放岗位级自定义权重与评分人校准。
5. **语音生态依赖浏览器**：Web Speech API 在非 Chromium 内核支持弱，完全跨平台一致体验需要统一服务端 ASR（成本换一致性）。
6. **测试金字塔右侧偏薄**：E2E 仅覆盖 happy path，实时房的断线/重连/并发场景仍以手工演练和单测为主，后续应补充 WS 集成测试与混沌场景自动化。
7. **部署形态**：目前以 compose 为主，K8s 清单、HPA、滚动发布与回滚流程尚未完整落地（计划见 RELEASE.md）。

---

*本文档随项目迭代更新；新增的关键问题请继续按"表现→分析→方案→效果经验"四段式追加，并在第 9 节补提交索引。*
