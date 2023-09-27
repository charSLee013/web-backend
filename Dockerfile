# 构建阶段
FROM golang:1.21.1-bookworm AS builder

WORKDIR /app

# 拷贝项目文件
COPY web-app/ ./

# 下载依赖
RUN go mod download

# 构建可执行文件
RUN CGO_ENABLED=0 go build -a -installsuffix cgo -o main .

# 运行阶段
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

# 从构建阶段拷贝可执行文件
COPY --from=builder /app/main .

# 设置执行权限
RUN chmod +x main

# 运行可执行文件
CMD ["./main"]