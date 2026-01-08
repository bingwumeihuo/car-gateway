# Build Stage
FROM golang:1.25-alpine AS builder

# 设置 Go 代理，加速依赖下载 (国内环境推荐)
ENV GOPROXY=https://goproxy.cn,direct

WORKDIR /app

# 复制依赖定义
COPY go.mod go.sum ./
RUN go mod download

# 复制源代码
COPY . .

# 编译服务端二进制文件
# CGO_ENABLED=0 确保静态链接，适合在 alpine/scratch 运行
RUN CGO_ENABLED=0 GOOS=linux go build -o vehicle-gateway-server cmd/server/main.go

# Run Stage
FROM alpine:latest

WORKDIR /app

# 安装必要的运行时依赖 (如时区数据)
RUN apk --no-cache add tzdata

# 设置时区为上海
ENV TZ=Asia/Shanghai

# 从构建阶段复制二进制文件
COPY --from=builder /app/car-gateway-server .

COPY --from=builder /app/configs ./configs

# 暴露端口 (与配置文件一致)
EXPOSE 8080

# 启动命令
CMD ["./car-gateway-server"]
