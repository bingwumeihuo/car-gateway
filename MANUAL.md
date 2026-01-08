# Vehicle Gateway 项目说明手册

## 1. 项目概述
本项目是一个基于 Go 语言开发的高性能新能源汽车接入网关，完全符合 **GB/T 32960.3-2025** 国家标准。项目采用 Clean Architecture 架构设计，支持高并发 TCP 连接、异步数据处理、实时数据解析与存储。

## 2. 系统架构

### 2.1 整体分层
*   **Infrastructure (基础设施层)**: 提供 Kafka 生产者 mock、TSDB 存储 mock 等底层实现。
*   **Protocol (协议层)**: 负责 GB/T 32960 报文的编解码、校验、粘包/拆包处理。
*   **UseCase (业务逻辑层)**:
    *   **Handler**: 核心业务调度，处理登录、实时数据、注销等指令。
    *   **SessionManager**: 管理车辆连接会话，支持心跳检测。
    *   **AuthService**: 负责 VIN 与 ICCID 的鉴权认证。
    *   **DataDispatcher**: 异步数据分发器，使用 Worker Pool 模式将解析后的数据并行投递到消息队列和数据库。
*   **Server (接入层)**: 基于 `gnet` 的高性能 TCP 服务端，管理网络生命周期。

### 2.2 核心特性
*   **异步架构**: TCP 解析与业务入库分离，解析后的数据通过 Buffered Channel 异步处理，避免阻塞网络线程。
*   **安全认证**: 强制要求终端登录，验证 VIN (用户名) 与 ICCID (密码) 的匹配性。
*   **协议全覆盖**: 支持整车数据、驱动电机、燃料电池、发动机、位置、极值、报警等全量数据类型的解析。
*   **健壮性**: 内置 PacketScanner 处理 TCP 粘包和断包问题，具备 BCC 校验纠错能力。

## 3. 功能模块详细说明

### 3.1 协议解析模块 (`internal/protocol`)
*   **Packet**: 定义了标准报文结构（起始符、命令单元、唯一标识、加密方式、数据单元、校验码）。
*   **PacketScanner**: 自定义 Buffer 扫描器，支持流式解析，能从碎片化 TCP 流中提取完整报文。

### 3.2 业务处理模块 (`internal/usecase`)
*   **Handler**:
    *   `CmdLogin (0x01)`: 校验车辆身份，拒绝非法连接。
    *   `CmdRealTime (0x02)`: 解析各类实时数据子包，并分发给 Dispatcher。
    *   `CmdLogout (0x03)`: 处理正常下线逻辑。
*   **Dispatcher**: 维护一个 worker pool (默认 100个 worker)，并行调用 `DataProducer` (Kafka) 和 `TimeSeriesRepo` (InfluxDB)。

### 3.3 测试客户端 (`internal/client`, `cmd/client`)
*   **PacketBuilder**: 封装了报文构建逻辑，自动填充时间、流水号、计算校验码。
*   **Client CLI**: 模拟真实车载终端行为，执行连接 -> 登录 -> 循环上报 -> 注销的完整流程。

## 4. 快速开始

### 4.1 环境要求
*   Go 1.25+

### 4.2 编译与运行
```bash
# 编译服务端
go build -o server_bin cmd/server/main.go

# 编译客户端
go build -o client_bin cmd/client/main.go

# 启动服务端
./server_bin
# 预期输出: [DataDispatcher] 启动了 100 个 Worker 协程... TCP Server is booting...

# 启动客户端 (另开终端)
./client_bin
# 预期输出: 登入成功! ... 发送实时数据 ...
```

### 4.3 配置说明
目前配置主要在代码中 `cmd/server/main.go` 及 `configs/config.yaml` (如有集成)。
默认监听端口: `8080`。
### 4.4 Docker 部署
提供了开箱即用的 Dockerfile，支持多阶段构建。

1. **构建镜像**
```bash
docker build -t vehicle-gateway:latest .
```

2. **运行容器**
```bash
# 默认使用 8080 端口
docker run -d -p 8080:8080 --name vg-server vehicle-gateway:latest
```

## 5. 鉴权说明
系统内置 `InMemoryAuthService` (位于 `internal/usecase/gbt32960/auth_service.go`)。
*   **白名单 VIN**: `VIN12345678901234`
*   **匹配 ICCID**: `12345678901234567890`
*   测试时请使用上述凭证，否则会报鉴权失败。

## 6. 目录结构
```
├── cmd
│   ├── client          # 客户端入口
│   └── server          # 服务端入口
├── internal
│   ├── client          # 客户端公共库 (PacketBuilder)
│   ├── config          # 配置定义
│   ├── infra           # 基础设施 (Kafka/TSDB Mock)
│   ├── protocol        # 协议解析 (Packet/Decoder)
│   ├── server          # TCP 服务端实现 (gnet)
│   └── usecase         # 业务逻辑 (Handler/Session/Auth/Dispatcher)
└── MANUAL.md           # 本手册
```
## 7. 编译Linux版本
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o server
