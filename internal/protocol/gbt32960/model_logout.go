package gbt32960

import (
	"encoding/binary"
	"errors"
	"time"
)

// LogoutData 车辆登出数据 (命令单元 0x03)
type LogoutData struct {
	CollectTime time.Time // 登出时间
	LogoutSeq   uint16    // 登出流水号
}

// ParseLogout 解析车辆登出数据
// 格式: [登出时间 6Byte][登出流水号 2Byte]
func ParseLogout(data []byte) (*LogoutData, error) {
	if len(data) < 8 {
		return nil, errors.New("登出数据长度不足")
	}

	year := int(data[0]) + 2000
	month := time.Month(data[1])
	day := int(data[2])
	hour := int(data[3])
	minute := int(data[4])
	second := int(data[5])

	loc, _ := time.LoadLocation("Asia/Shanghai")
	t := time.Date(year, month, day, hour, minute, second, 0, loc)

	seq := binary.BigEndian.Uint16(data[6:8])

	return &LogoutData{
		CollectTime: t,
		LogoutSeq:   seq,
	}, nil
}
