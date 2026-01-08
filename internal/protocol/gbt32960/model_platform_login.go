package gbt32960

import (
	"errors"
)

// PlatformLoginData 平台登入数据 (命令单元 0x05)
type PlatformLoginData struct {
	Username string
	Password string
}

func ParsePlatformLogin(data []byte) (*PlatformLoginData, error) {
	// Minimum expected length: 6 + 2 + 12 + 20 + 1 = 41
	if len(data) < 40 { // Allow strict or loose? Hex len was 41. Let's start with check for User+Pass fields.
		return nil, errors.New("平台登入数据长度不足 (期望>=41)")
	}

	offset := 6 + 2 // Skip Time(6) and Seq(2)

	// Username: 12 Bytes
	if len(data) < offset+12 {
		return nil, errors.New("用户名数据不足")
	}
	userBytes := data[offset : offset+12]
	// Remove trailing nulls
	username := string(trimNulls(userBytes))
	offset += 12

	// Password: 20 Bytes
	if len(data) < offset+20 {
		return nil, errors.New("密码数据不足")
	}
	passBytes := data[offset : offset+20]
	password := string(trimNulls(passBytes)) // Usually password might not have nulls if it matches length, but safe to trim
	offset += 20

	return &PlatformLoginData{
		Username: username,
		Password: password,
	}, nil
}

func trimNulls(b []byte) []byte {
	for i := len(b) - 1; i >= 0; i-- {
		if b[i] != 0 {
			return b[:i+1]
		}
	}
	return []byte{}
}
