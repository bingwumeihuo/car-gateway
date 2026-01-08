package gbt32960

import (
	"go.uber.org/zap"
	"sync"
	"time"
)

type Conn interface {
	RemoteAddr() string
	Close() error
	Write([]byte) (int, error)
	SetPlatformAuthenticated(bool)
	IsPlatformAuthenticated() bool
}

// Session 代表一个车辆连接会话
type Session struct {
	VIN            string
	Conn           Conn
	LastActiveTime time.Time // 最后活跃时间
	LoginTime      time.Time // 登入时间
}

// SessionManager 管理车辆会话
type SessionManager struct {
	sessions sync.Map // map[string]*Session (VIN -> Session)
	logger   *zap.Logger
}

// NewSessionManager 创建一个新的会话管理器
func NewSessionManager(logger *zap.Logger) *SessionManager {
	return &SessionManager{
		logger: logger,
	}
}

// Add 为指定 VIN 创建或更新会话
func (sm *SessionManager) Add(vin string, conn Conn) {
	session := &Session{
		VIN:            vin,
		Conn:           conn,
		LastActiveTime: time.Now(),
		LoginTime:      time.Now(),
	}
	sm.sessions.Store(vin, session)
	sm.logger.Info("[SessionManager] Session Added", zap.String("vin", vin), zap.String("remote_addr", conn.RemoteAddr()))
}

// Remove 删除会话
func (sm *SessionManager) Remove(vin string) {
	if val, ok := sm.sessions.LoadAndDelete(vin); ok {
		sess := val.(*Session)
		sm.logger.Info("[SessionManager] Session Removed", zap.String("vin", sess.VIN))
		_ = sess.Conn.Close()
	}
}

// Get 获取会话
func (sm *SessionManager) Get(vin string) (*Session, bool) {
	val, ok := sm.sessions.Load(vin)
	if !ok {
		return nil, false
	}
	return val.(*Session), true
}

// UpdateLastActive 更新会话的心跳时间
func (sm *SessionManager) UpdateLastActive(vin string) {
	if val, ok := sm.sessions.Load(vin); ok {
		sess := val.(*Session)
		sess.LastActiveTime = time.Now()
	}
}

// CheckHeartbeat 检查过期的会话并关闭它们。
func (sm *SessionManager) CheckHeartbeat(timeout time.Duration) {
	now := time.Now()
	sm.sessions.Range(func(key, value interface{}) bool {
		sess := value.(*Session)
		if now.Sub(sess.LastActiveTime) > timeout {
			sm.logger.Info("[SessionManager] Session Timeout", zap.String("vin", sess.VIN), zap.Duration("inactive_duration", now.Sub(sess.LastActiveTime)))
			sm.Remove(sess.VIN)
		}
		return true // 继续遍历
	})
}
