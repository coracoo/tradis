# deployments/Dockerfile
# 构建后端
FROM golang:1.22-alpine AS backend-builder
# 设置代理
ENV HTTP_PROXY=http://192.168.0.129:7890
ENV HTTPS_PROXY=http://192.168.0.129:7890
WORKDIR /app
COPY backend/ .
RUN CGO_ENABLED=0 go build -o main ./cmd

# 最终阶段
FROM alpine:latest

# 安装必要的包
RUN apk add --no-cache nginx supervisor

# 复制前端构建文件
COPY frontend/dist /usr/share/nginx/html

# 复制后端二进制
COPY --from=backend-builder /app/main /app/main

# 复制配置文件
COPY deployments/nginx.conf /etc/nginx/nginx.conf
COPY deployments/supervisord.conf /etc/supervisord.conf

# 创建必要的目录和权限
RUN mkdir -p /var/log/nginx /var/log/supervisor && \
    chown -R nginx:nginx /var/log/nginx && \
    chmod +x /app/main

# 映射 docker sock
VOLUME /var/run/docker.sock

# 暴露端口
EXPOSE 80

# 使用 supervisor 管理进程
CMD ["/usr/bin/supervisord", "-c", "/etc/supervisord.conf"]