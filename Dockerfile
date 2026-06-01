# Yunxi Home v3.0.0 — 多阶段构建
# 构建: docker build -t yunxi-home .
# 多架构: docker buildx build --platform linux/amd64,linux/arm64 -t yunxi-home .

# Stage 1: 构建前端
FROM node:20-alpine AS frontend-builder
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci --silent
COPY web/ ./
RUN npm run build

# Stage 2: 构建后端
FROM golang:1.24-alpine AS backend-builder
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
COPY --from=frontend-builder /src/web/dist ./internal/web/static/
RUN --mount=type=cache,target=/root/.cache/go-build CGO_ENABLED=0 \
    go build -ldflags="-s -w" -o /yunxi-home ./cmd/yunxi-home/

# Stage 3: 最终运行镜像
FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata && \
    cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime && \
    echo "Asia/Shanghai" > /etc/timezone

RUN adduser -D -g '' dnsupdater
USER dnsupdater

WORKDIR /app
COPY --from=backend-builder /yunxi-home .

RUN mkdir -p /app/data /app/log

EXPOSE 9981

VOLUME ["/app/configs", "/app/data", "/app/log"]

ENTRYPOINT ["/app/yunxi-home"]
CMD ["-config", "/app/configs/config.yaml"]
