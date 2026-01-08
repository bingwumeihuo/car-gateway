package gbt32960

import (
	"encoding/binary"
	"errors"
)

// ExtremeData 极值数据 (类型 0x06)
type ExtremeData struct {
	MaxVoltageSubSysID byte    // 最高电压电池子系统号
	MaxVoltageProbeID  byte    // 最高电压电池单体代号
	MaxVoltage         float32 // 电池单体电压最高值 (V), 精度0.001
	MinVoltageSubSysID byte    // 最低电压电池子系统号
	MinVoltageProbeID  byte    // 最低电压电池单体代号
	MinVoltage         float32 // 电池单体电压最低值 (V), 精度0.001
	MaxTempSubSysID    byte    // 最高温度子系统号
	MaxTempProbeID     byte    // 最高温度探针序号
	MaxTemp            byte    // 最高温度值 (℃), 偏移40
	MinTempSubSysID    byte    // 最低温度子系统号
	MinTempProbeID     byte    // 最低温度探针序号
	MinTemp            byte    // 最低温度值 (℃), 偏移40
}

// ParseExtremeData 解析极值数据
func ParseExtremeData(data []byte) (*ExtremeData, error) {
	if len(data) < 14 {
		return nil, errors.New("极值数据长度不足")
	}

	maxVolt := binary.BigEndian.Uint16(data[2:4])
	minVolt := binary.BigEndian.Uint16(data[6:8])

	return &ExtremeData{
		MaxVoltageSubSysID: data[0],
		MaxVoltageProbeID:  data[1],
		MaxVoltage:         float32(maxVolt) * 0.001,
		MinVoltageSubSysID: data[4],
		MinVoltageProbeID:  data[5],
		MinVoltage:         float32(minVolt) * 0.001,
		MaxTempSubSysID:    data[8],
		MaxTempProbeID:     data[9],
		MaxTemp:            data[10] - 40,
		MinTempSubSysID:    data[11],
		MinTempProbeID:     data[12],
		MinTemp:            data[13] - 40,
	}, nil
}
