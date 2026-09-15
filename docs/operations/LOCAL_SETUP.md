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

# 3. 数据库迁移

迁移文件位于 `backend/migrations`，已通过 go:embed 编译进二进制。
使用独立的迁移命令（推荐）：

```bash
cd backend
go run ./cmd/migrate up       # 应用全部待执行迁移
go run ./cmd/migrate status   # 查看当前版本与待执行迁移
```

迁移记录在 `schema_migrations` 表，每个版本只应用一次（事务内执行 + 记录版本）。
up 迁移均使用 `IF NOT EXISTS` / `ON CONFLICT`，因此在历史手工建库（无版本表）上首次执行也安全：
会幂等重跑全部迁移并补记版本。

也可显式指定目录（排查/运维）：

```bash
go run ./cmd/migrate -dir ./migrations up
```

容器部署时迁移已内嵌，无需挂载 migrations 目录：

```bash
./server-migrate up
```

历史手工脚本（不推荐，仅应急）：

```powershell
Get-ChildItem "backend\migrations\*.up.sql" | Sort-Object Name | ForEach-Object {
  Write-Host "applying $($_.Name)"
  Get-Content $_.FullName -Raw | docker exec -i ai-interview-pg psql -U postgres -d ai_interview -f -
}
```

注意：迁移文件必须完整执行（建表 + 索引），遗漏索引会导致检索性能问题。

---

# 4. 后端

```bash
cd backend
go test ./...
go run ./cmd/server
```

---

# 5. 前端

```bash
cd frontend
npm install
npm run dev
```

---

# 6. 完整启动

```bash
docker compose up -d
```

具体服务以当前 compose 文件为准。
