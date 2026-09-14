# 本地开发

## 1. 环境

需要：

```text
Go
Node.js
Docker
Docker Compose
```

---

# 2. 启动基础设施

```bash
docker compose up -d postgres redis minio
```

---

# 3. 后端

```bash
cd backend
go test ./...
go run ./cmd/server
```

---

# 4. 前端

```bash
cd frontend
npm install
npm run dev
```

---

# 5. 完整启动

```bash
docker compose up -d
```

具体服务以当前 compose 文件为准。
