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

迁移文件位于 `backend/migrations`，按文件名顺序手动应用。
迁移均使用 `IF NOT EXISTS`，可重复执行（幂等）。

```powershell
# 应用全部 up 迁移（PowerShell）
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
