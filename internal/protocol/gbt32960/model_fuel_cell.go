package gbt32960

import (
	"encoding/binary"
	"errors"
)

// FuelCellData 燃料电池数据 (类型 0x03)
type FuelCellData struct {
	Voltage         float32 // 燃料电池电压 (V), 精度0.1
	Current         float32 // 燃料电池电流 (A), 精度0.1
	FuelConsumeRate float32 // 燃料消耗率 (kg/100km), 精度0.01
	TempProbeCount  uint16  // 燃料电池温度探针总数
	ProbeTemps      []byte  // 探针温度值 (℃), 偏移40℃
	MaxTemp         float32 // 氢系统中最高温度 (℃), 偏移40, 精度0.1 (标准2016版由 2+2+2+2+N + 2+1+1+1 = 6+N+6? 2025版可能有变化，此处按常规结构实现，需对照最新2025pdf)
}

// ParseFuelCellData 解析燃料电池数据
func ParseFuelCellData(data []byte) (*FuelCellData, error) {
	// 最小长度: 2+2+2+2 = 8
	if len(data) < 8 {
		return nil, errors.New("燃料电池数据长度不足")
	}

	voltRaw := binary.BigEndian.Uint16(data[0:2])
	currRaw := binary.BigEndian.Uint16(data[2:4])
	rateRaw := binary.BigEndian.Uint16(data[4:6])
	count := binary.BigEndian.Uint16(data[6:8])

	expectedLen := 8 + int(count) // 假设每个探针 1 字节
	if len(data) < expectedLen {
		return nil, errors.New("燃料电池探针数据长度不足")
	}

	temps := make([]byte, count)
	// 偏移 8 开始读取 N 个字节
	for i := 0; i < int(count); i++ {
		temps[i] = data[8+i] - 40 // 偏移40
	}

	return &FuelCellData{
		Voltage:         float32(voltRaw) * 0.1,
		Current:         float32(currRaw) * 0.1,
		FuelConsumeRate: float32(rateRaw) * 0.01,
		TempProbeCount:  count,
		ProbeTemps:      temps,
	}, nil
}
