package gbt32960

import (
	"encoding/binary"
	"errors"
)

// LocationData 车辆位置数据 (类型 0x05)
type LocationData struct {
	State     byte    // 定位状态 (位 0:有效/无效, 位 1:南/北纬, 位 2:东/西经)
	Longitude float64 // 经度, 精度 1e-6
	Latitude  float64 // 纬度, 精度 1e-6
}

// ParseLocationData 解析位置数据
func ParseLocationData(data []byte) (*LocationData, error) {
	if len(data) < 9 {
		return nil, errors.New("位置数据长度不足")
	}

	// 状态(1) + 经度(4) + 纬度(4) = 9
	longRaw := binary.BigEndian.Uint32(data[1:5])
	latRaw := binary.BigEndian.Uint32(data[5:9])

	return &LocationData{
		State:     data[0],
		Longitude: float64(longRaw) / 1000000.0,
		Latitude:  float64(latRaw) / 1000000.0,
	}, nil
}
