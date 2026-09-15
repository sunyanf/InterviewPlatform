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
