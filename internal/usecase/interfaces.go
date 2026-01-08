package usecase

import "context"

// Conn 抽象连接接口
type Conn interface {
	RemoteAddr() string
	Close() error
	Write([]byte) (int, error)
	// State management
	SetPlatformAuthenticated(bool)
	IsPlatformAuthenticated() bool
}

type DataProducer interface {
	// Produce 发送数据到指定 Topic
	Produce(ctx context.Context, topic string, key string, data interface{}) error
}
