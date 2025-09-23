# 使用轻量级 Go 镜像
FROM golang:1.21-alpine

# 设置工作目录
WORKDIR /app

# 拷贝 Go 文件
COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY . .

# 构建可执行文件
RUN go build -o server main.go

FROM alpine:latest
WORKDIR /root/
COPY --from=builder /app/server .
EXPOSE 3000
CMD ["./server"]