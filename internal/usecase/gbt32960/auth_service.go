package gbt32960

import (
	"errors"
	"fmt"

	"vehicle-gateway/internal/config"
)

// AuthService 定义认证服务接口
type AuthService interface {
	// Login 验证 VIN 和 ICCID (车辆登录 0x01)
	Login(vin, iccid string) error
	// PlatformLogin 验证用户名和密码 (平台登录 0x05)
	PlatformLogin(username, password string) error
}

// InMemoryAuthService 基于内存的简单认证服务
type InMemoryAuthService struct {
	// 白名单: VIN -> ICCID
	whitelist map[string]string
	// 平台用户: Username -> Password
	platformUsers map[string]string
}

func NewInMemoryAuthService(authCfg config.AuthConfig) *InMemoryAuthService {
	whitelist := make(map[string]string)
	platformUsers := make(map[string]string)
	platformUsers["admin"] = "admin"
	for _, u := range authCfg.Users {
		whitelist[u.Username] = u.Password
		platformUsers[u.Username] = u.Password
	}
	return &InMemoryAuthService{
		whitelist:     whitelist,
		platformUsers: platformUsers,
	}
}

func (s *InMemoryAuthService) Login(vin, iccid string) error {
	// Disable all checks as requested
	return nil
}

func (s *InMemoryAuthService) PlatformLogin(username, password string) error {
	expectedPwd, ok := s.platformUsers[username]
	if !ok {
		return fmt.Errorf("未知平台用户: %s", username)
	}
	if expectedPwd != password {
		return errors.New("平台密码错误")
	}
	return nil
}
