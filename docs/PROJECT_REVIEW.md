# AI 面试模拟平台 · 全生命周期复盘与改进审查报告

> 范围：从项目初始化（commit `25eda68`）至当前（commit `745f2e1`，共 50 个提交）的完整开发历程。
> 方法：所有结论基于实际代码、提交历史、ADR 与运行验证；改进建议均给出文件位置、实施步骤与优先级，可直接执行。
> 结构：**一、挑战与解决（技术 + 非技术，按项目阶段组织）｜二、关键经验教训｜三、改进建议（按 P0/P1/P2 分级）**。
> 配套文档：[INTERVIEW_RETROSPECTIVE.md](./INTERVIEW_RETROSPECTIVE.md)（15 个技术问题的 STAR 详述）、[ARCHITECTURE.md](./ARCHITECTURE.md)、[adr/](./adr/)。

---

# 一、挑战与解决（Challenges & Resolutions）

## 阶段划分与交付物总览

| 阶段 | 周期（MVP 计划） | 提交区间 | 核心交付 |
|---|---|---|---|
| P1 基础设施与业务骨架 | Week 1–3 | `25eda68`–`50708f3` | Go 骨架、Auth、PostgreSQL/MinIO、岗位、简历、面试会话状态机 |
| P2 AI 能力构建 | Week 4–7 | `f8a5c90`–`0d8b549` | Agent 六角色、RAG+pgvector、Rubric 评估、报告 |
| P3 实时化与语音 | Week 8–9 | `db6a72b`–`e8f16b5` | WebSocket 面试房、ASR/TTS、流式对话、断线恢复 |
| P4 工程化与产品闭环 | Week 10–12 及之后 | `50fa39f`–`745f2e1` | 异步任务、限流/令牌轮换、可观测、E2E、管理后台、体验修复 |

---

## A. 技术挑战

### 挑战 T1（P1 阶段）：本地基础设施起不来——镜像拉取失败与端口冲突

**问题陈述与影响**
- 开发机在国内网络，DockerHub 拉取 MinIO 官方镜像超时，`docker-compose up` 直接卡住，阻塞 Week 1 所有联调（预计影响半天到一天工期）；本机已装 Redis 占用 6379，compose 端口冲突导致依赖启动失败；CI 与本地 Go 小版本不一致曾导致构建行为差异（commit `e575ae4`）。

**解决步骤**
1. MinIO 改用 `quay.io/minio/minio` 镜像源（commit `38342f3`）；
2. compose 中 Redis 映射到 6380，避让开发机已有服务；
3. 统一 Go 版本为 1.24.1（CI 与 go.mod 对齐）；
4. 所有环境差异通过 `.env` + godotenv 收敛，沉淀 `docs/operations/LOCAL_SETUP.md` 标准启动文档。

**收获的知识**
- 容器化交付必须预判"开发者机器不是干净的"：端口、镜像源、代理都要可配置；
- 基础设施问题要文档化为"一条命令可复现"的标准解法，而不是口口相传。

---

### 挑战 T2（P1–P2 阶段）：LLM 输出不可靠——脏 JSON、空结果、限流与超时

**问题陈述与影响**
- 模型返回被 ` ```json ` 代码块包裹、字段缺失、choices 为空时，出题/评估直接失败；高峰期 429 与网络抖动导致整条 AI 链路报错。若不治理，AI 功能的成功率无法支撑产品演示，评估结果也不可信。相关代码：[pkg/llm/openai.go](../backend/pkg/llm/openai.go)、[internal/agent/agent.go](../backend/internal/agent/agent.go)。

**解决步骤**
1. Provider 层重试分级：仅对网络错误（status=0）和 429/5xx 重试 2 次、200ms 指数退避；4xx 立即失败，尊重 ctx 取消；
2. 非流式请求 60s 整体超时；流式请求不设整体超时、完全由 ctx 控制；
3. Agent 层 `cleanJSON` 剥离 markdown 围栏 → 强类型 struct 解析（`unmarshal`）→ 枚举/范围业务校验 → nil 切片归一化，再允许消费；
4. 补齐 AI 异常矩阵测试：空输出、字段缺失、类型错误、超时、重试次数（commit `f8a5c90`）。

**收获的知识**
- LLM 是系统中最不可靠的外部依赖，必须按第三方支付接口的标准做容错：错误分级、重试边界、超时治理；
- "结构化输出"不是 prompt 里写一句"请输出 JSON"就结束，而是**清洗 + Schema + 业务校验**的完整管线。

---

### 挑战 T3（P2 阶段）：评分主观性——AI 直接给总分不可信、不可回归

**问题陈述与影响**
- 若让模型输出"综合 78 分"，同一回答多次评分会波动，用户无法理解分从何来；改 prompt 后可能悄悄劣化评分质量，且无任何手段发现。直接影响产品核心价值（反馈可信度）。

**解决步骤**
1. 确立 ADR-0004：LLM 不拥有业务状态与最终分数；
2. LLM 只按 Rubric 四维（技术正确性 0.30、深度 0.25、逻辑 0.25、表达 0.20）输出**维度分 + 证据引用**，总分由代码加权求和（`CalculateTotalScore`，保留两位小数），权重合法性由 `ValidateRubric` 强制（和为 1）；
3. Rubric 快照随评估落库，可追溯历史评分口径；
4. 建黄金问答数据集 + evaluator 回归 runner（`evalrun` 包，commit `4332ca3`），prompt/模型变更必须跑回归。

**收获的领域知识**
- AI 系统的信任来自"人类可审计的证据链 + 确定性代码兜底最终结论"；
- Eval 数据集是 AI 应用的"单元测试"，没有回归集的 prompt 迭代等于盲改。

---

### 挑战 T4（P2 阶段）：RAG 知识来源不明、时效性与无结果降级

**问题陈述与影响**
- 面试/考试知识（尤其公务员政策）时效性强，来源不明或过期材料一旦进入 prompt，模型会自信引用，风险外溢到评分结论；检索无召回时 nil 处理不当还可能引发异常。代码：[internal/knowledge/service.go](../backend/internal/knowledge/service.go)、[chunker.go](../backend/internal/knowledge/chunker.go)。

**解决步骤**
1. 摄取强校验：source 必填（否则 `INVALID_SOURCE` 拒绝）、source_type 限定 manual/official/interview_bank/docs 四枚举、effective_from/to 必须合法 RFC3339；
2. 分块保证语义完整：段落聚合目标 500 字、超长段按句子边界切分、上限 800 字；
3. 检索按有效期与来源过滤，nil 结果归一化为空切片；无召回时显式降级为纯模型作答，prompt 要求只依据提供材料；
4. 知识行落 collected_at/version（commit `8500726`）。

**收获的领域知识**
- RAG 项目 80% 的风险在数据治理（来源、时效、分块质量），不在向量算法本身；
- pgvector 一库承担关系+向量（ADR-0003）在 MVP 规模下显著降低运维与双写一致性成本。

---

### 挑战 T5（P3 阶段，最具深度）：实时面试房三大故障——心跳断连、断线丢答案、重连状态错乱

**问题陈述与影响**
用户在面试房中弱网操作时连环踩坑，直接影响核心场景可用性：
1. 提交回答后页面卡死约 60 秒，随后 WS 被服务端断开——读循环被数十秒的 LLM 调用阻塞，无法消费 pong，触发 60s 读空闲超时（commit `1d274c5`）；
2. 网络抖动或多标签顶号（close 4000）后，已提交回答丢失、永远没有分析——链路沿用了 WS 连接 ctx，连接一断整条"落库+分析"被取消（commit `f20a107`）；
3. 重连后答题气泡重复或永久"分析中"——服务端只推 snapshot 不补事件帧，断线期间的 answer_saved/analysis 事件永久丢失，前端乐观事件无对账机制（commit `a1d7112`）。

**解决步骤（三层修复）**
- **连接层**：readPump 只收帧与派发，answer/chat 异步 goroutine 处理；`answerInFlight/chatInFlight` 原子闸保证同类耗时消息一轮在途；所有写帧经独立 writePump（gorilla 单写者约束）+ 64 缓冲 channel，慢消费者关连接；
- **业务层**：`SubmitAnswer` 改为"**先落库 → 再尽力分析 → 再决策追问**"三段；全程 `context.WithoutCancel` 脱离连接生命周期，分别配 70s 总超时 / 30s 分析 / 20s 追问超时兜底；分析失败只打点不阻断，并显式下发 `analysis_skipped` 终态；
- **恢复层**：连接即推全量 snapshot；前端 `reconcileFromSession` 按 questionId 把服务端事实与本地占位事件对账（复用 key 防重复气泡）；悬置事件用"只排一次"的 REST 轮询补拉；busy 状态拆三阶段且每个进行中状态都有可达终态。

**收获的知识**
- 长连接协议必须把"协议心跳循环"与"业务慢操作"隔离，心跳路径禁止不可控耗时；
- **连接 ctx ≠ 业务 ctx**：用户已确认提交的意图数据先持久化，衍生型 AI 加工降级为尽力而为；
- 实时系统的重连恢复要以"服务端全量事实"为基线对账本地乐观状态，事件丢失是常态；
- 单端互踢要处理"旧连接延迟死亡"竞态：`Hub.clear` 用指针相等判断、HubManager 引用计数双重检查后再回收。

---

### 挑战 T6（P3 阶段）：浏览器语音识别抖动、静默断听与跨浏览器兼容

**问题陈述与影响**
- Web Speech API 的 interim 结果会反复修正甚至回退，逐帧渲染导致文字跳动；静默数秒后浏览器自动 `onend` 停止识别，用户还在说却不再转写；不按 resultIndex 去重会重复拼接；非 Chromium 内核基本不可用。代码：[VoiceRecorder.tsx](../frontend/src/interview/VoiceRecorder.tsx)。

**解决步骤**
1. 文本状态拆 `committed`（final 永不回退）+ `interim`（临时可覆盖）两段渲染；
2. 按 `resultIndex` 增量消费结果去重（commit `9e0c2a1`）；
3. `onend` 且非用户主动停止时静默自动重启，重启计数封顶防死循环；最大时长 timer + watchdog 双收尾，`finishedRef` 保证只收尾一次；卸载清理全部定时器与流；
4. SR 不可用/失败降级 MediaRecorder 录音上传服务端 ASR（commit `f7a9360`、`e279d9a`）；TTS 音频落 MinIO 返回预签名 URL 并缓存。

**收获的知识**
- 非规范化浏览器能力必须读懂事件语义并准备服务端兜底；乐观 UI 要区分"已定稿"与"暂定稿"；
- 语音方案要算成本账：输入优先免费的浏览器识别、上传 ASR 兜底，输出按需合成+缓存。

---

### 挑战 T7（P4 阶段）：长耗时 AI 任务同步阻塞、重启即丢

**问题陈述与影响**
- 简历解析、评估、报告均为数十秒级 LLM 任务，同步 HTTP 会被网关/浏览器超时打断；服务重启时在途任务半途而废；前端只能干等。代码：[internal/task/runner.go](../backend/internal/task/runner.go)。

**解决步骤**
1. 新建 tasks 表（类型/payload/attempts/租约 owner/状态/last_error），API 入队即返 `202 + task_id`（commit `ce58926`、`6f43e73`）；
2. bounded worker pool 用 `FOR UPDATE SKIP LOCKED` 抢占；租约 5min + reaper 回收僵死任务；失败指数退避（10s 起步、上限 2min）；超 max_attempts 置 failed 并触发业务补偿钩子（如简历回写 failed）；
3. 业务处理器全部幂等 Upsert；优雅关闭有界等待，任务 ctx 故意不继承 runner 取消信号（可做到租约上限）；
4. 前端 react-query 凭 task_id 轮询（commit `7afffab`）；`TASK_ENABLED` 支持 API/worker 分进程部署（commit `d73d2d8`）。

**收获的知识**
- 没有 MQ 也能做出可靠异步：PG 行锁队列 + 租约/退避/回收四件套；"至少一次"投递模型下幂等是业务方必修课；
- 不为不存在的需求（削峰、多消费者、跨服务事件）提前引入 Kafka，是对"过早复杂度"的刻意克制。

---

### 挑战 T8（P4 阶段）：安全基线——令牌重放、暴力登录、跨站 WS、敏感信息入日志

**问题陈述与影响**
- 初版刷新令牌长期不轮换，泄露即可无限使用；登录无限流可撞库；WS 无 Origin 校验存在 CSWSH；query token 可能被日志泄露。任何一项在真实环境都是高危。

**解决步骤**（commit `b925943`、`538ba90`）
1. 刷新令牌服务端落库、**一次性轮换**，重用可被发现；
2. 限流中间件登录类按 IP 10 次/分钟、业务 API 按用户 120 次/分钟，正确解析客户端 IP；
3. WS 升级强制 `ALLOWED_ORIGINS` 白名单，prod 环境不允许为空（config.Validate 强制）；
4. 安全响应头（nosniff / DENY / Referrer-Policy）、请求体大小限制（JSON 1MB、上传 12MB）、日志对 token/密码脱敏、密码 bcrypt；
5. 前端 401 静默刷新 + 单飞重放，避免多请求并发刷新（commit `6546c2f`）。

**收获的知识**
- 安全必须在基础设施期进入中间件链，事后补成本高；
- 浏览器 WS 无法设 Authorization 头，query token 方案必须配套 wss + Origin 白名单 + 日志脱敏。

---

### 挑战 T9（产品闭环期）：真实使用暴露的三个数据契约/可用性缺陷

**问题陈述与影响**
- ① 首页"已答"全为 0、岗位名为空：列表 SQL 只投影了 session 主表，详情接口才有统计与 JOIN（commit `d0ab7de`）；② 点空岗位分类整页白屏：Go nil 切片序列化为 `null`，前端对 null 取 `.length` 崩溃（commit `661e984`）；③ 管理后台全英文 key、用户找不到新增岗位入口，产品不可自助使用。

**解决步骤**
1. 列表 SQL 用 LEFT JOIN + 相关子查询一次补齐岗位标题与回答数（避免 N+1）；
2. 双端防线：Service 层 nil 归一化为 `[]`，前端全面可选链兜底；空态页直接放"自定义岗位"CTA；
3. 岗位页新增"粘贴 JD 创建岗位"表单（后端 `POST /jobs` 本就开放），创建后直达面试准备页；管理后台三项分页全部中文标注 + 每项配置中文名与填写说明（commit `4803c75`、`6716e85`）。

**收获的知识**
- 跨语言序列化契约必须显式约定（集合字段永远返 `[]` 而不是 null）；
- 修显示问题先抓真实接口响应，区分数据源问题还是渲染问题；
- 空态页不是终点而是功能入口；后台/配置类界面必须面向"非开发用户"写说明。

---

## B. 非技术挑战

### 挑战 N1：单人开发 + AI 辅助编码下的需求快速演进与质量失控风险

**问题陈述与影响**
- 项目由个人借助 AI Coding Agent 高速迭代，最大风险是"改得快但失控"：顺手重构、AI 幻觉式修改、状态被模型输出左右、测试被删来迁就代码。若不约束，代码库会在 2–3 周内不可维护。

**解决步骤**
1. 制定仓库级《Agent 宪法》（AGENTS.md，29 条）：明确优先级规则（P0 平台安全 > 人工指令 > 宪法 > ADR > 代码）、最小修改原则、禁止事项清单（删测试迁就代码、让 LLM 控制状态等）、Done 定义与强制汇报格式；
2. 长期决策进 ADR（4 份）、产品事实进 PROJECT_MEMORY、临时任务进 docs/tasks；
3. 约定每个独立任务一个 conventional commit 并即时提交，保持变更可回溯、可回滚；
4. AI 变更（Prompt/Schema/Rubric）有登记模板（AI_CHANGELOG、DECISION_LOG）。

**过程收获**
- 人负责目标与不可逆决策、Agent 负责分析与实现、代码与数据库负责运行时事实——三方职责划清后，AI 辅助开发才能既快又稳；
- "规则文档化 + 每任务小提交"是单人项目控制复杂度的最低成本手段。

### 挑战 N2：AI 产品的"信任设计"是业务问题不是技术问题

- 用户是否相信 AI 评分、是否敢把练习交给模型，决定产品留存。技术解法（Rubric、Evidence、版本化、Eval 回归、维度分明细展示）本质上是在回答一个业务问题："这个分数凭什么"。收获：AI 产品要把"可解释性"当一等功能设计，而不是事后加注释。

### 挑战 N3：范围与架构克制——计划里有 Kafka/K8s/微服务，实际刻意不做

- MVP_PLAN 的 Week 11/12 列了 Kafka、Kubernetes；实际用 PG 任务表、compose 交付。取舍依据是"当前是否有真实负载证据"。收获：架构选型最大的敌人是简历驱动开发（resume-driven development）；先画好拆分缝（模块边界、接口、TASK_ENABLED），等指标证明瓶颈再迁移，比一步到位更难也更正确。

### 挑战 N4：12 周计划的节奏管理与垂直切片策略

- 按周排期但坚持"每周端到端可演示"：Week 1 即打通 Auth→DB→HTTP，而不是先写三周工具层。收获：垂直切片让每周末都有可验收交付物，风险（LLM/语音/WS）全部前置暴露，后期集成风险被摊薄。

### 挑战 N5：真实用户反馈闭环（本轮三连缺陷的组织性根因）

- "已答 0、黑屏、后台看不懂、找不到入口"全部是真实使用才暴露的问题，代码评审无法发现。收获：必须建立"用户试用 → 录屏/描述 → 抓真实接口响应定位 → 修复 → 浏览器端到端复验"的闭环；且验证要在修复后真实重启服务、清缓存复测，不能只看单测绿。

---

# 二、关键经验教训（Key Lessons Learned）

## 2.1 架构与设计

1. **确定性边界是 AI 应用的第一性原理**：状态机、计时、权限、事务、总分、配额归代码；理解与生成归模型。这条原则让所有"AI 灵异行为"都有明确归属层。
2. **Modular Monolith + 预留拆分缝**：包边界 + Service 接口 + 接口适配（`knowledgeRetriever`、`evaluationKnowledge`）让未来拆服务时改装配而非改逻辑。
3. **一库多用途服务于当前规模**：PostgreSQL + pgvector 省掉向量库运维与双写一致性；访问向量的代码收敛在 repository 一层，替换成本可控。
4. **抽象只在有第二个实现时引入**：LLM Provider 抽象有 mock/openai/多家兼容厂商三个实现支撑；其余地方坚持标准库（自写限流器、worker pool、重试）。

## 2.2 可靠性与并发

5. **长连接三条军规**：心跳路径不做重活；所有写走单写泵；每个"进行中"状态必须有可达终态（含 skipped/timeout）。
6. **context 语义分层**：请求 ctx 随连接生死，业务 ctx 用 `WithoutCancel` + 显式超时；流式调用 ctx 绑定连接以便断线即停、省 token。
7. **异步任务可靠性公式**：幂等 Upsert + 租约 + 退避 + 僵死回收 + 失败补偿，缺一不可；优雅关闭时在途任务不被强杀，靠租约兜底。
8. **数据先落库、AI 后加工**：把用户意图数据与衍生数据的优先级分开，是断线不丢答案的根本设计。

## 2.3 AI 工程化

9. **AI 输出管线化**：清洗 → Schema → 业务校验 → 归一化；禁止模型字符串直接入库。
10. **Prompt 是代码**：版本号随结果落库（`prompt_version`、`model` 字段），变更要能回滚、要过黄金数据集回归。
11. **降级体系显式化**：回答必保、分析可补、追问可无、RAG 可空、TTS 失败不影响文字——AI 子能力故障不应拖垮主流程。
12. **成本可观测**：token 用量与估算费用进 Prometheus（MeteredProvider），没有计量就无法管理模型成本。

## 2.4 流程与协作（含 AI 协作）

13. **证据优先于推断**：修 bug 先抓真实接口响应、日志与复现路径；写文档只引用代码中存在的常量、文件与提交。
14. **小步提交 + 即时验证**：一任务一 commit；gofmt/vet/tsc/build/定向测试是每次修改后的固定动作。
15. **测试对准"系统面对 AI 异常是否安全"**：大量构造空输出/缺字段/超时/重试用例，比测"模型聪不聪明"有价值得多。
16. **非开发视角验收**：空态、错误文案、后台说明、入口可发现性与接口正确性同等重要。

## 2.5 可量化的工程参数（均为代码常量，可在面试中直接引用）

| 维度 | 数值 | 位置 |
|---|---|---|
| WS 心跳/读超时/发送缓冲/消息上限 | ping 50s、pongWait 60s、buffer 64、1MB | realtime/hub.go、handler.go |
| 答题后台链路超时 | 总 70s / 分析 30s / 追问 20s | interview/model.go |
| 异步任务 | worker 2、轮询 2s、租约 5min、退避 10s→2min、关闭等待 20s | task/runner.go、config.go |
| LLM 调用 | 默认 30s、HTTP 60s、重试 2 次、退避 200ms 起 | agent.go、openai.go |
| 限流 | 登录 10/min/IP、API 120/min/用户 | config.go |
| 请求体 | JSON 1MB、上传 12MB | config.go |
| RAG 分块 | 500 字目标 / 800 字上限 | knowledge/chunker.go |
| 前端产物体积 | gzip JS ≈ 86KB | vite build |
| 直接依赖数 | 后端 10 个 | go.mod |

---

# 三、改进建议（Improvement Recommendations）

> 优先级：**P0 = 安全/数据正确性，应尽快修；P1 = 可靠性/性能/合规，近期排期；P2 = 体验与效率优化**。每条含：证据 → 影响 → 可直接执行的步骤 → 预期收益。

## 3.1 安全增强

### P0-1　知识库写/删接口缺少管理员授权（越权）
- **证据**：[server.go 第 269–272 行](../backend/internal/server/server.go) 中 `POST/DELETE /knowledge/documents` 仅在普通 Auth 组内，任何登录用户都可写入和**删除全局知识库**——而知识库内容会进入所有用户的出题与评估 prompt。
- **影响**：普通用户可投毒/删除共享 RAG 数据，污染全站面试题与评分依据，属于水平越权 + 供应链式内容风险。
- **步骤**：① 将 Create/Delete 移入 `RequireAdmin` 组（与 /admin/* 同级）；② 若未来允许用户自建私有知识，给 documents 加 owner_id + visibility(global/private)，检索时按用户隔离；③ 补一条"普通用户调用返回 403"的中间件/handler 测试。
- **预期收益**：消除越权与内容投毒面。

### P0-2　Access Token 有效期偏长（2 小时）
- **证据**：[config.go 第 176 行](../backend/internal/config/config.go) `JWT_EXPIRE_TIME` 默认 2h，且 token 存 localStorage。
- **影响**：XSS 或终端泄露后窗口期长；localStorage 对任意 JS 可读。
- **步骤**：① access TTL 缩短至 15 分钟（已有静默刷新，用户无感）；② 评估将刷新令牌改为 httpOnly + Secure + SameSite=Strict cookie，前端只保留内存 access token；③ 增加 refresh 令牌家族（token family）重用检测：同一家族内旧 refresh 被重用即整族吊销（防盗用）。
- **预期收益**：令牌泄露窗口从 2h 缩到 15min，并消除 JS 读取刷新令牌的可能。

### P1-3　/metrics 无鉴权暴露
- **证据**：[server.go 第 201 行](../backend/internal/server/server.go) 注释依赖"网络策略限制"，但默认无任何校验。
- **影响**：指标中含路径、任务类型、LLM 用量，可辅助攻击者侦察。
- **步骤**：① 加可选 Bearer（`METRICS_TOKEN`）中间件，未配置时仅监听内网/localhost；② 或在反向代理层用独立端口暴露并限制来源 IP；③ 确认指标标签不含 user_id/邮箱等 PII。
- **预期收益**：缩小信息泄露面，满足基线合规扫描。

### P1-4　文件上传仅按扩展名白名单
- **证据**：[resume/service.go 第 68 行](../backend/internal/resume/service.go) 只校验 `filepath.Ext` 与声明大小。
- **影响**：可上传伪装成 .pdf 的恶意内容；解析库遇到畸形文件可能 panic/资源耗尽。
- **步骤**：① 读取前 512 字节用 `http.DetectContentType` 校验魔数（PDF `%PDF`、docx 的 zip 头 PK、text 为 text/plain）；② PDF 解析加单文件页数/大小上限与解析超时；③ 对象存储 key 使用 UUID 而非用户提供文件名，Content-Disposition 固定下载。
- **预期收益**：阻断恶意文件投递，降低解析器攻击面。

### P1-5　HTTP Server 缺少 IdleTimeout
- **证据**：[server.go 第 174–179 行](../backend/internal/server/server.go) 只设置 Read/WriteTimeout。
- **影响**：慢速连接（Slowloris 风格）可长期占用连接与 goroutine。
- **步骤**：增加 `IdleTimeout: 120s`（注意 WS 已 Hijack 不受影响，SSE/长轮询如有需单独评估）；同步暴露为 `SERVER_IDLE_TIMEOUT` 配置。
- **预期收益**：以一行配置消除慢速连接资源占用。

### P1-6　限流器为单机内存实现，多副本部署即失真
- **证据**：`SecurityConfig.RateLimitEnabled` 注释已自述"多实例需改 Redis 版"；compose 已部署 Redis。
- **影响**：API/worker 分进程或水平扩容后，登录撞库防护按实例数被稀释。
- **步骤**：用 Redis Lua 脚本实现固定窗口/令牌桶（原子 INCR+EXPIRE），Limiter 接口不变、按配置切换内存/Redis 实现；补集成测试。
- **预期收益**：水平扩容时限流语义保持一致。

### P2-7　其他安全硬化清单
- 审计日志：管理后台的设置变更、用户禁用操作落审计表（谁、何时、改了什么），当前仅有普通日志；
- 账号防护：登录连续失败后对账号维度加指数退避/临时锁定（当前仅 IP 维度，攻击者换 IP 可绕过）；
- `DB_SSLMODE` 默认 disable：prod 环境 Validate 中强制 require/verify-full；
- 密钥轮换：LLM api_key 等 settings 更新保留最近版本与轮换记录。

## 3.2 性能与容量

### P1-8　每个请求重复构造 Handler/Repository/Service
- **证据**：[server.go 第 220–303 行](../backend/internal/server/server.go) 路由注册中直接调用 `s.userHandler()`、`s.jobHandler()` 等，**每次请求都 new repository**；`realtimeHandler()`/`ttsHandler()` 还各自 new 了独立的 interview.Service（WS 与 REST 不是同一实例）。
- **影响**：功能无错（pool 共享），但产生无谓分配；更重要的是同一领域存在多个 service 实例，未来在 service 上加内存缓存/状态时会出现多副本不一致。
- **步骤**：在 `New()` 中一次性构造各 handler 存为 Server 字段，路由引用字段；interview service 单例化供 WS/REST/TTS 共用；补 `go test -race ./...`。
- **预期收益**：零功能风险的微优化 + 消除多实例隐患。

### P1-9　面试列表无分页
- **证据**：`GET /interviews` 走 `ListByUserID` 返回该用户全部会话（[repository.go 第 70 行](../backend/internal/interview/repository.go)）。
- **影响**：重度用户几百场面试后响应体与首页渲染时间线性增长。
- **步骤**：复用岗位列表的分页响应契约（page/page_size/total，list 永不为 null），首页只取第一页 + 游标/分页器；SQL 用 keyset 分页（created_at,id）优于 OFFSET。
- **预期收益**：列表延迟与数据量解耦。

### P1-10　AI 成本与并发缺少配额控制
- **证据**：答题分析、自由对话、评估均可触发 LLM 调用，现只有限流（120/min），无"每用户并发 AI 请求数"与"每日 token 预算"。
- **影响**：单用户可通过刷自由对话制造高额 token 账单（DoS 式成本攻击）。
- **步骤**：① 复用 WS 的在途标志思路，在业务层给每用户 chat 设并发上限；② 基于已有 LLM 计量指标增加每日 token 预算（Redis 计数），超额返回 429 + 友好提示；③ 自由对话历史做 token 预算裁剪（只带最近 N 轮 + 摘要）。
- **预期收益**：成本可控，防滥用。

### P2-11　数据库与检索优化
- 核对并补齐索引：`interview_answers(session_id)`、`tasks(status, lease_until)`、`knowledge_chunks` 的 ivfflat/HNSW 参数随数据量调优（上线前用 EXPLAIN ANALYZE 验证）；
- RAG 增加**混合检索**：pgvector 余弦相似度 + 全文检索（tsvector / BM25 近似）分数融合，再加一次轻量 rerank，显著提升关键词类考题召回；
- 知识检索结果可加短时缓存（同 query 向量 + topK），降低 embedding 与检索成本。

## 3.3 可靠性与可运维性

### P1-12　WebSocket 关键事件未持久化
- **证据/现象**：当前重连恢复依赖全量 snapshot + 前端对账，断线期间流式回复的中断点无法精确续传。
- **步骤**：① 将 answer_saved/analysis/follow_up/chat_done 等关键事件追加写入 session 事件表（或 outbox 表）；② 连接建立时支持 `?last_event_id=` 增量补拉 + snapshot 兜底；③ 设置保留期（如 24h）定期清理。
- **预期收益**：弱网/刷新后的恢复从"最终一致"提升到"过程可续"，并为后期审计留痕。

### P2-13　启动期故障策略不一致
- **证据**：provider 初始化失败直接 `panic`（[server.go 第 87 行](../backend/internal/server/server.go)）；而 settings.Load 失败仅记日志后继续运行。
- **步骤**：统一启动策略：必需依赖失败→拒绝启动并非零退出（readyz 不通过）；可降级依赖（如非默认 LLM provider）→显式降级标记 + 启动横幅告警；为 settings 加载失败增加重试与启动失败开关。
- **预期收益**：故障行为可预测，避免"带病启动"。

### P2-14　运维工作流
- CI 增加：`golangci-lint`、`go test -race`、前端 eslint、覆盖率上报（先观测不设阈值，两周后再定门槛）；
- pre-commit 钩子跑 gofmt + tsc，降低低级返工；
- 部署：compose 已有全栈与 Prometheus，下一步补蓝绿/滚动发布脚本、数据库迁移在发布流水线中自动执行（`cmd/migrate` 已独立）、回滚 runbook；
- 告警：基于现有指标先配 3 条核心告警——LLM 失败率 >10%、任务 failed 速率、readyz 失败；
- 密钥管理：生产 MinIO/JWT/LLM 密钥从 .env 迁移到 secret manager 或至少文件权限受控的 docker secret。

## 3.4 用户体验（UX）

### P1-15　登录失败反馈与限流提示
- **现象/证据**：认证限流为 10 次/分钟（config.go），测试期多次试错即被限流，但前端未必能区分"密码错误"与"请求过多"。
- **步骤**：统一错误码到中文文案映射（429→"尝试过于频繁，请 N 秒后再试"并展示倒计时；401→"邮箱或密码错误"）；登录页加"忘记密码/联系管理员"路径；对被禁用账号给出明确提示。
- **预期收益**：减少用户在登录环节的流失与困惑。

### P2-16　面试房体验细化
- 全局 React Error Boundary + 路由级 loading 骨架屏（当前异常依赖各页 spinner）；
- 面试房移动端适配核查（语音按钮、流式气泡、输入框在小屏的可达性）；
- "分析中"给出可理解的进度文案（"正在对照考点分析你的回答…"）而非转圈；
- 断线状态条：区分"重连中/已重连，正在恢复答题记录"，恢复成功给轻提示；
- 评估报告支持导出 PDF/分享链接（预签名只读 URL），便于用户复习。

### P2-17　空态与引导系统化
- 把本次"空分类→创建岗位 CTA"的模式推广到：无简历（引导上传并说明解析用途）、无面试（引导选岗）、无评估报告（说明完成面试后生成）；
- 首次登录加 3 步引导浮层（选岗→传简历→开始模拟）。

## 3.5 开发与 AI 协作效率

### P2-18　测试金字塔补强
- 当前 E2E 仅 happy path（commit `adce5e0`）。补：① WS 集成测试（断线重连、顶替、在途拒绝）；② 仓库层测试用 testcontainers 起真实 PG 验证 SKIP LOCKED 抢占与 reaper；③ 前端关键 reducer（useInterviewSocket 对账逻辑）抽出纯函数做单测。
- 黄金评估集纳入 CI 必跑项并保存历次评分漂移对比。

### P2-19　API 契约工程
- 从 Go struct 生成 OpenAPI 文档（或用代码标注），前端基于生成类型，避免本次"list:null"这类跨语言契约问题靠人肉发现；
- 集合字段在 response 包层面做统一保证（可选：封装 successList 渲染器），从框架层杜绝 null 数组。

### P2-20　可观测性闭环
- 接入 Grafana 预置看板（HTTP RED、任务成功率/延迟、LLM 成本、WS 在线连接数/顶替次数）；
- trace：request_id 已贯穿日志，可进一步接入 OpenTelemetry，把"出题→RAG→LLM→落库"串成一条 trace，定位 AI 慢调用具体卡在哪一跳。

## 3.6 建议实施路线图（建议顺序）

| 迭代 | 内容 | 理由 |
|---|---|---|
| 第 1 批（1–2 天） | P0-1 知识库授权、P0-2 token TTL、P1-5 IdleTimeout、P1-8 handler 单例 | 改动小、安全收益高、无产品影响 |
| 第 2 批（3–5 天） | P1-3 metrics 鉴权、P1-4 上传魔数、P1-15 登录文案、P2-19 集合契约统一 | 安全基线 + 即时体验 |
| 第 3 批（1–2 周） | P1-6 Redis 限流、P1-9 列表分页、P1-10 AI 配额、P2-18 WS/任务测试 | 面向扩容与成本 |
| 第 4 批（按产品节奏） | P1-12 事件持久化、P2-11 混合检索、P2-14/16/17/20 运维与体验 | 竞争力增强 |

---

## 附：本次审查的方法与边界

- **证据来源**：全部后端 internal 包关键文件、前端关键页面与 ws hook、go.mod/package.json、4 份 ADR、50 个提交、运行期接口实测与浏览器端到端验证。
- **未覆盖（需补充环境）**：真实压测下的 p95/p99 延迟与 PG 连接池占用、生产部署的网络策略与 secret 管理实况、移动端真机兼容性——这些已在建议中标注验证方法，未做无依据推断。
