# 第一阶段: 构建Go程序
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY main.go .
RUN go mod init xunlei-config && \
    CGO_ENABLED=0 GOOS=linux go build -o web-server

# 第二阶段: 最终镜像
FROM python:3.9-alpine

WORKDIR /app

# 安装基本工具和Python依赖
RUN apk add --no-cache bash && \
    pip install requests

# 复制编译好的Go程序
COPY --from=builder /app/web-server /app/web-server

# 复制Python脚本
COPY swjsq.py /app/

# 创建数据目录
RUN mkdir /data && \
    chmod 777 /data

# 声明数据卷
VOLUME ["/data"]

# 暴露web服务端口
EXPOSE 8080

# 直接启动Go程序
CMD ["/app/web-server"]
