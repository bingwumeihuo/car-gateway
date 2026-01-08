package gbt32960

import (
	"encoding/binary"
	"errors"
)

// MotorUnit 单个驱动电机数据
type MotorUnit struct {
	Seq      byte    // 电机序号
	Status   byte    // 电机状态 (0x01:耗电, 0x02:发电, 0x03:关闭, 0x04:准备)
	CtrlTemp byte    // 控制器温度 (℃), 偏移40℃ (0=-40℃)
	Speed    int16   // 电机转速 (r/min), 偏移20000 (0=-20000) -> 实际上是有符号还是无符号偏移? 标准说是 偏移量20000r/min。范围0~65531表示-20000~45531
	Torque   float32 // 电机转矩 (N·m), 偏移2000N·m, 精度0.1
	Temp     byte    // 电机温度 (℃), 偏移40℃
	Voltage  float32 // 电机控制器输入电压 (V), 精度0.1
	Current  float32 // 电机控制器直流母线电流 (A), 偏移1000A, 精度0.1
}

// MotorData 驱动电机数据 (类型 0x02)
type MotorData struct {
	Count     byte        // 电机个数
	MotorList []MotorUnit // 电机列表
}

// ParseMotorData 解析驱动电机数据
func ParseMotorData(data []byte) (*MotorData, error) {
	if len(data) < 1 {
		return nil, errors.New("电机数据为空")
	}
	count := data[0]
	// 每个电机数据长度 12 字节
	// 序号(1) + 状态(1) + 控制器温度(1) + 转速(2) + 转矩(2) + 温度(1) + 电压(2) + 电流(2) = 12
	expectedLen := 1 + int(count)*12
	if len(data) < expectedLen {
		return nil, errors.New("电机数据长度不匹配")
	}

	list := make([]MotorUnit, 0, count)
	offset := 1
	for i := 0; i < int(count); i++ {
		chunk := data[offset : offset+12]

		speedRaw := binary.BigEndian.Uint16(chunk[3:5])
		torqueRaw := binary.BigEndian.Uint16(chunk[5:7])
		voltRaw := binary.BigEndian.Uint16(chunk[8:10])
		currRaw := binary.BigEndian.Uint16(chunk[10:12])

		unit := MotorUnit{
			Seq:      chunk[0],
			Status:   chunk[1],
			CtrlTemp: chunk[2] - 40,
			// 转速: 0 表示 -20000
			Speed: int16(int(speedRaw) - 20000),
			// 转矩: 0 表示 -2000, 精度 0.1
			Torque:  (float32(torqueRaw) * 0.1) - 2000.0,
			Temp:    chunk[7] - 40,
			Voltage: float32(voltRaw) * 0.1,
			// 电流: 0 表示 -1000, 精度 0.1
			Current: (float32(currRaw) * 0.1) - 1000.0,
		}
		list = append(list, unit)
		offset += 12
	}

	return &MotorData{
		Count:     count,
		MotorList: list,
	}, nil
}
