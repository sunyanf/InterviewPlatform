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
