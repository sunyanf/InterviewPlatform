# 部署指南（MVP 单机生产形态）

## 1. 架构

```text
浏览器
  ↓ HTTP/WS
frontend 容器（nginx：SPA 静态资源 + /api 反代）
  ↓
backend 容器（HTTP API + 异步任务 worker，/metrics 指标）
  ↓                ↘
postgres+pgvector   MinIO（简历/音频/语音文件，bucket 自动创建）
```

Redis 当前代码零引用，不纳入生产部署；后续分布式限流/队列启用时再加入。

## 2. 启动

```bash
cp .env.production.example .env.production
# 编辑 .env.production：ALLOWED_ORIGINS、DB/STORAGE 密码、JWT_SECRET（openssl rand -hex 32）

docker compose -f docker-compose.prod.yml --env-file .env.production up -d --build
```

启动顺序由 compose 保证：postgres 健康检查 → `migrate up` 一次性迁移（成功退出）
→ backend（含 worker）→ frontend。

验证：

```bash
curl http://localhost/healthz                 # 经 nginx → backend
curl http://localhost:8080/readyz             # 直连本机绑定端口，校验 PG/MinIO
```

## 3. API / Worker 分离（多副本）

backend 单进程同时承担 HTTP 与异步任务。横向扩容时复制该服务：

- API 节点：`TASK_ENABLED=false`（只服务 HTTP，可多副本）；
- Worker 节点：`TASK_ENABLED=true`（消费任务，任务领取依赖租约行锁，勿全部关闭）。

任务领取基于 PostgreSQL `FOR UPDATE SKIP LOCKED` 租约，多 worker 安全；
限流为单机内存令牌桶（`RATE_LIMIT_ENABLED`），多副本前需切换为集中式实现。

## 4. 监控

```bash
docker compose -f docker-compose.prod.yml --env-file .env.production \
  --profile monitoring up -d prometheus
```

Prometheus 控制台仅绑定 `127.0.0.1:9090`，抓取 `backend:8080/metrics`；
指标清单见 [OBSERVABILITY.md](OBSERVABILITY.md)。Grafana/告警在后续阶段补齐。

## 5. 压测

[k6](https://k6.io/) 只读冒烟脚本（登录接口有 10 次/分/IP 限流，不参与压测；
token 预先人工获取一次）：

```bash
# 1. 登录取 token（API 返回 data.token）
# 2. 执行（PowerShell 用 $env: 传参）
k6 run -e K6_TOKEN=<token> -e BASE_URL=http://localhost:8080 deploy/loadtest/k6-smoke.js
```

阈值基线（本机 mock 链路）：错误率 < 1%，p95 < 500ms。
真实 Provider 链路的超时/重试压测在阶段 B 完成。

## 6. 升级与回滚

```bash
git pull
docker compose -f docker-compose.prod.yml --env-file .env.production up -d --build
# 迁移随启动自动执行；破坏性迁移需人工确认（AGENTS.md #18）
```

回滚：切换镜像 tag / git revert 后重新 up；迁移不可逆时须提前备份
`pgdata` 卷（`pg_dump`）。
