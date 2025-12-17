# Build frontend
FROM node:18-alpine AS frontend-builder
WORKDIR /app/frontend
COPY frontend/package*.json ./
RUN npm install
COPY frontend .
RUN npm run build

# Build backend
FROM golang:1.24-alpine AS backend-builder
WORKDIR /app
RUN apk add --no-cache build-base
COPY go.mod go.sum ./
RUN go mod download
COPY backend ./backend
# 使用 CGO 编译静态二进制文件 (支持 sqlite)
RUN CGO_ENABLED=1 GOOS=linux go build -ldflags="-s -w -extldflags '-static'" -o server backend/cmd/main.go

# Final stage
FROM alpine:latest
WORKDIR /app

# 安装必要的系统工具
RUN apk --no-cache add ca-certificates tzdata

# 复制后端二进制
COPY --from=backend-builder /app/server .
# 复制前端构建产物
COPY --from=frontend-builder /app/frontend/dist ./dist
# 创建上传目录
RUN mkdir -p uploads

# 环境变量
ENV PORT=3002
ENV GIN_MODE=release

# 暴露端口
EXPOSE 3002

CMD ["./server"]
