package gbt32960

import (
	"encoding/binary"
)

// EngineData 发动机数据 (类型 0x04)
type EngineData struct {
	Status   byte    // 发动机状态 (0x01:启动, 0x02:关闭)
	Speed    uint16  // 曲轴转速 (r/min)
	FuelRate float32 // 燃料消耗率 (L/100km), 精度0.01
}

// ParseEngineData 解析发动机数据
func ParseEngineData(data []byte) (*EngineData, error) {
	//if len(data) < 5 {
	//	return nil, errors.New("发动机数据长度不足")
	//}

	// 状态(1) + 转速(2) + 消耗率(2) = 5
	speed := binary.BigEndian.Uint16(data[1:3])
	rate := binary.BigEndian.Uint16(data[3:5])

	return &EngineData{
		Status:   data[0],
		Speed:    speed,
		FuelRate: float32(rate) * 0.01,
	}, nil
}
