# 开发规范

## 1. 分层

推荐：

```text
Handler
 ↓
Service
 ↓
Repository
```

Agent / LLM 属于独立能力层。

---

# 2. 代码组织

```text
backend/
├── cmd/
├── internal/
│   ├── user/
│   ├── job/
│   ├── resume/
│   ├── interview/
│   ├── agent/
│   ├── knowledge/
│   ├── evaluation/
│   ├── report/
│   └── learning/
├── pkg/
│   ├── llm/
│   ├── embedding/
│   ├── asr/
│   ├── tts/
│   ├── storage/
│   └── observability/
└── migrations/
```

---

# 3. Repository

Repository 只负责：

- 数据访问；
- 查询；
- 持久化。

不应该：

- 做 Prompt；
- 做业务编排；
- 修改 HTTP Response。

---

# 4. Service

Service 负责：

- 业务规则；
- 事务；
- 权限；
- 状态转换。

---

# 5. Handler

Handler 负责：

- 参数解析；
- 鉴权入口；
- 调用 Service；
- 返回统一响应。

---

# 6. LLM

LLM Adapter 负责：

- Provider 调用；
- 超时；
- token；
- model；
- raw response。

业务层不应依赖供应商 SDK。

---

# 7. 提交前检查

```bash
gofmt
go test ./...
go test -race ./...
go vet ./...
```

视项目工具配置继续运行 lint。

---

# 8. E2E（Playwright）

前端 `frontend/e2e/` 固化两条主链路：

- happy path：注册 → 选岗 → 上传简历 → 建面试 → 逐题作答 → 结束 → 评估报告；
- 失败态：评估失败时报告页展示错误态，「重新生成」会重新发起评估请求。

本地运行（后端 :8080 + PostgreSQL + MinIO 需已启动）：

```bash
cd frontend
npm ci
npx playwright install chromium
npx playwright test                 # 自动拉起/复用 5173 dev server
# 或指向已运行的前端：
$env:E2E_BASE_URL="http://localhost:5174"; npx playwright test   # PowerShell
E2E_BASE_URL=http://localhost:5174 npx playwright test           # bash
```

CI（`.github/workflows/ci.yml` 的 `e2e` job）自动启动 pgvector 与 MinIO 服务、
执行迁移并运行全栈用例；产物（backend 日志、playwright-report）在失败时上传 artifact。

---

# 9. 真实 Provider 联调（DeepSeek 等 OpenAI 兼容厂商）

LLM Provider 走标准 `/chat/completions` 协议，切换厂商只改 env，不改代码。
DeepSeek 不提供 embedding / ASR / TTS，这三项 MVP 阶段保持 mock：
知识检索的向量通道退化为关键词（FTS）通道，语音为 mock 转写/合成。

1. 编辑 `backend/.env`：填入 `LLM_API_KEY`，并将 `LLM_PROVIDER` 改为 `openai`
   （base_url/model 的 DeepSeek 预设已写好；key 不要入库、不要贴进对话）。
2. 结构输出契约回归（39 条用例真实调用，验证 JSON 契约与边界鲁棒性）：

   ```bash
   cd backend && go run ./cmd/evalrunner
   ```

3. 模型/金标评分一致性（容差 15 分、通过率阈值 0.8）：

   ```bash
   go run ./cmd/evalrunner -agreement -tolerance 15 -min-agreement 0.8
   ```

4. 真实业务链路：重启 server 后跑一次 Playwright happy path（出题/分析/评估全部真实），
   并检查 `/metrics` 中 `llm_requests_total{model="deepseek-chat",status="success"}`、
   `llm_tokens_total`、`llm_cost_total` 记账。

失败排查：401 检查 key；`llm api error status=400` 检查 model 名与请求参数；
结构解析失败先看 evalrunner 输出的具体 case（prompt 版本需按 AGENTS.md #13 升版）。
