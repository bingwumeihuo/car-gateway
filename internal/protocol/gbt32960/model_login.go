package gbt32960

import (
	"encoding/binary"
	"time"
)

// LoginData 车辆登入数据 (命令单元 0x01)
type LoginData struct {
	CollectTime time.Time // 数据采集时间
	LoginSeq    uint16    // 登入流水号
	Password    string    // ICCID (20字节)
	SubSysCount byte      // 可充电储能子系统数
	Coding      byte      // 可充电储能系统编码长度
}

// ParseLogin 解析车辆登入数据
// 格式: [采集时间 6Byte][登入流水号 2Byte][ICCID 20Byte][子系统数 1Byte][编码长度 1Byte]
func ParseLogin(data []byte) (*LoginData, error) {
	//if len(data) < 30 {
	//	return nil, errors.New("登入数据长度不足")
	//}

	// 解析时间 (6字节: 年 月 日 时 分 秒)
	year := int(data[0]) + 2000
	month := time.Month(data[1])
	day := int(data[2])
	hour := int(data[3])
	minute := int(data[4])
	second := int(data[5])

	loc, _ := time.LoadLocation("Asia/Shanghai")
	t := time.Date(year, month, day, hour, minute, second, 0, loc)

	seq := binary.BigEndian.Uint16(data[6:8])
	password := string(data[8:28]) // ICCID

	return &LoginData{
		CollectTime: t,
		LoginSeq:    seq,
		Password:    password,
		SubSysCount: data[28],
		Coding:      data[29],
	}, nil
}
