package server

import (
	"context"
	"fmt"
	"strings"

	"github.com/panjf2000/gnet/v2"

	"go.uber.org/zap"

	"vehicle-gateway/internal/config"
	protocol "vehicle-gateway/internal/protocol/gbt32960"
	handler "vehicle-gateway/internal/usecase/gbt32960"
)

// connContext 保存每个连接的状态
type connContext struct {
	buffer           []byte
	scanner          *protocol.PacketScanner
	addr             string
	isPlatformAuthed bool
}

type GnetConnWrapper struct {
	conn gnet.Conn
}

func (w *GnetConnWrapper) RemoteAddr() string {
	return w.conn.RemoteAddr().String()
}

func (w *GnetConnWrapper) Close() error {
	return w.conn.Close()
}

func (w *GnetConnWrapper) Write(b []byte) (n int, err error) {
	return w.conn.Write(b)
}

func (w *GnetConnWrapper) SetPlatformAuthenticated(v bool) {
	ctx, ok := w.conn.Context().(*connContext)
	if ok {
		ctx.isPlatformAuthed = v
	}
}

func (w *GnetConnWrapper) IsPlatformAuthenticated() bool {
	ctx, ok := w.conn.Context().(*connContext)
	if ok {
		return ctx.isPlatformAuthed
	}
	return false
}

type TCPServer struct {
	gnet.BuiltinEventEngine

	addr      string
	multicore bool
	logger    *zap.Logger
	handler   *handler.Handler
}

func NewTCPServer(cfg *config.Config, logger *zap.Logger, h *handler.Handler) *TCPServer {
	return &TCPServer{
		addr:      fmt.Sprintf("tcp://%s:%d", cfg.Server.Host, cfg.Server.Port),
		multicore: true,
		logger:    logger,
		handler:   h,
	}
}

func (s *TCPServer) OnBoot(eng gnet.Engine) (action gnet.Action) {
	s.logger.Info("TCP Server is booting", zap.String("address", s.addr))
	return
}

func (s *TCPServer) OnOpen(c gnet.Conn) (out []byte, action gnet.Action) {
	s.logger.Info("New connection opened", zap.String("remote_addr", c.RemoteAddr().String()))

	// 初始化连接上下文
	ctx := &connContext{
		buffer:  make([]byte, 0, 4096),
		scanner: protocol.NewPacketScanner(65535), // 最大包长 64KB
		addr:    c.RemoteAddr().String(),
	}
	c.SetContext(ctx)

	return
}

func (s *TCPServer) OnTraffic(c gnet.Conn) (action gnet.Action) {
	ctx := c.Context().(*connContext)

	// 读取新数据
	buf, _ := c.Next(-1)
	if len(buf) > 0 {
		// 追加到连接缓冲区
		ctx.buffer = append(ctx.buffer, buf...)

		// Parse Packets Loop
		for {
			advance, token, err := ctx.scanner.SplitFunc(ctx.buffer, false)
			if err != nil {
				s.logger.Error("Packet split error", zap.Error(err), zap.String("addr", ctx.addr))
				action = gnet.Close
				return
			}

			if advance > 0 && token == nil {
				// 跳过垃圾数据或错误起始符，推进缓冲区
				ctx.buffer = ctx.buffer[advance:]
				continue
			}

			if token != nil {
				// 获取到有效的报文数据
				// 我们需要在这里解析头部或直接转换字节为 Packet 结构体
				// SplitFunc 返回的是原始字节流。
				// 我们必须手动构建 Packet 结构体或使用辅助函数。

				pkt, err := parseRawPacket(token)
				if err != nil {
					s.logger.Warn("Failed to parse packet struct", zap.Error(err))
				} else {
					// 调用业务 Handler
					// 包装连接
					wrapper := &GnetConnWrapper{conn: c}
					if err := s.handler.HandleMessage(wrapper, pkt); err != nil {
						s.logger.Warn("Handle message failed", zap.Error(err), zap.String("vin", pkt.VIN))
					}
				}

				// 推进缓冲区
				ctx.buffer = ctx.buffer[advance:]
				continue
			}

			// 需要更多数据
			break
		}
	}

	return
}

func (s *TCPServer) OnClose(c gnet.Conn, err error) (action gnet.Action) {
	s.logger.Info("Connection closed", zap.String("remote", c.RemoteAddr().String()), zap.Error(err))
	// 手动清理逻辑?
	// SessionManager 逻辑通过 Login/Logout/Heartbeat 处理移除。
	// 如果连接异常断开，SessionManager 最终会超时移除。
	// 可选: 我们可以在 context 中映射 Conn -> VIN 并在此时调用 Remove(vin)。
	return
}

func (s *TCPServer) OnShutdown(eng gnet.Engine) {
	s.logger.Info("TCP Server is shutting down")
}

func (s *TCPServer) Start(ctx context.Context) error {
	s.logger.Info("Starting TCP Server", zap.String("addr", s.addr))
	return gnet.Run(s, s.addr,
		gnet.WithMulticore(s.multicore),
		gnet.WithLogger(s.logger.Sugar()),
		gnet.WithReusePort(true),
	)

}

func (s *TCPServer) Stop(ctx context.Context) error {
	s.logger.Info("Stopping TCP Server...")
	return gnet.Stop(context.Background(), s.addr)
}

// parseRawPacket 将原始有效帧字节转换为 Packet 结构体
func parseRawPacket(data []byte) (*protocol.Packet, error) {
	// 结构: [Start 2][Cmd 1][Resp 1][VIN 17][Enc 1][Len 2][Data N][Check 1]
	// Header Fixed Size = 24
	if len(data) < 25 { // Header(24) + Checksum(1) = 25 Min
		return nil, fmt.Errorf("packet too short")
	}

	cmd := data[2]
	resp := data[3]
	vin := strings.TrimRight(string(data[4:21]), "\x00 ")
	enc := data[21]

	// 长度位于 22,23。
	// 数据单元从 24 开始。结束于 len-1 (Checksum)。
	dataUnit := data[24 : len(data)-1]

	// 创建 DataUnit 副本以防万一
	duCopy := make([]byte, len(dataUnit))
	copy(duCopy, dataUnit)

	var ver protocol.ProtocolVersion
	if data[0] == 0x23 && data[1] == 0x23 {
		ver = protocol.Version2016
	} else if data[0] == 0x24 && data[1] == 0x24 {
		ver = protocol.Version2025
	} else {
		// Should have been filtered by Scanner, but for safety
		ver = protocol.Version2016 // Default? Or error? PacketScanner ensures valid start chars.
	}

	return &protocol.Packet{
		Version:      ver,
		Command:      cmd,
		Response:     resp,
		VIN:          vin,
		Encryption:   enc,
		DataUnit:     duCopy,
		OriginalData: nil,
	}, nil
}
