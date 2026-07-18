# Build stage
FROM golang:1.25-alpine AS builder

# 安装必要工具
RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /app

# 复制 go.mod 和 go.sum 并下载依赖（利用缓存
COPY go.mod go.sum ./
RUN go mod download

# 复制源代码
COPY . .

# 构建
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/nmsappsrv ./cmd/main.go

# Runtime stage
FROM alpine:latest

# 安装运行时依赖
RUN apk add --no-cache ca-certificates tzdata \
    && ln -sf /usr/share/zoneinfo/Asia/Shanghai /etc/localtime \
    && echo "Asia/Shanghai" > /etc/timezone

WORKDIR /app

# 从构建阶段复制二进制文件
COPY --from=builder /app/nmsappsrv /app/nmsappsrv

# 复制配置文件
COPY configs/config.yaml /app/configs/config.yaml

# 创建数据和日志目录
RUN mkdir -p /app/data /app/logs /app/cert

# 暴露端口
# 8080 - HTTP API / TR-069 ACS / WebSocket / WebSSH
# 50000 - TR-069 UDP Connection Request
# 162 - SNMP Trap (需要 --net=host 或 privileged)
# 10022 - ZTP SFTP
EXPOSE 8080 50000/udp 162/udp 10022

# 健康检查
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# 启动命令
ENTRYPOINT ["/app/nmsappsrv"]
CMD ["--config", "/app/configs/config.yaml"]
