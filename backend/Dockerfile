# syntax=docker/dockerfile:1

# ---- build stage ----
FROM golang:1.24-alpine AS builder

WORKDIR /build

# 国内构建环境默认 Go proxy 被墙；构建参数可覆盖：--build-arg GOPROXY=...
ARG GOPROXY=https://goproxy.cn,direct
ENV GOPROXY=${GOPROXY}

# 先拉依赖以利用层缓存（go.mod/go.sum 变化少时跳过重新下载）
COPY go.mod go.sum ./
RUN go mod download

# 复制源码（迁移 SQL 经 go:embed 编入二进制，无需在运行镜像中携带）
COPY . .

# 静态构建：CGO_ENABLED=0 使镜像可基于 distroless/scratch 运行
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server \
    && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/migrate ./cmd/migrate

# ---- runtime stage ----
FROM alpine:3.20

# ca-certificates: HTTPS 调用 LLM/ASR/TTS Provider；tzdata: 时区
# wget(busybox): 容器 HEALTHCHECK
RUN apk add --no-cache ca-certificates tzdata wget \
    && addgroup -S app && adduser -S app -G app

WORKDIR /app
COPY --from=builder /out/server /app/server
COPY --from=builder /out/migrate /app/migrate

USER app

EXPOSE 8080

# 存活探针；就绪探针 /readyz 由编排平台按需配置
HEALTHCHECK --interval=15s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -q -O /dev/null http://127.0.0.1:8080/livez || exit 1

# 默认运行 API 服务；迁移由编排以 `migrate up` 单独执行（一次性 job/init 容器）
CMD ["/app/server"]
