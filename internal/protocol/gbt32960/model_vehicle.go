package gbt32960

import (
	"encoding/binary"
	"errors"
)

// VehicleData 整车数据 (类型 0x01)
type VehicleData struct {
	Status        byte    // 车辆状态 (0x01:启动, 0x02:熄火, 0x03:其他)
	ChargeStatus  byte    // 充电状态 (0x01:停车充电, 0x02:行驶充电, 0x03:未充电, 0x04:充电完成)
	RunMode       byte    // 运行模式 (0x01:纯电, 0x02:混动, 0x03:燃油)
	Speed         float32 // 车速 (km/h) 精度0.1
	TotalMileage  float64 // 累计里程 (km) 精度0.1
	Voltage       float32 // 总电压 (V) 精度0.1
	Current       float32 // 总电流 (A) 精度0.1, 偏移量1000A (0=-1000A)
	SOC           byte    // SOC (%)
	DCStatus      byte    // DC-DC状态 (0x01:工作, 0x02:断开)
	Gear          byte    // 挡位 (位掩码)
	InsulationRes uint16  // 绝缘电阻 (kΩ)
	AccelPedal    byte    // 加速踏板行程值 (%) (0-100)
	BrakePedal    byte    // 制动踏板行程值 (%) (0-100)
}

// ParseVehicleData 解析整车数据 (20字节)
func ParseVehicleData(data []byte) (*VehicleData, error) {
	if len(data) < 20 {
		return nil, errors.New("整车数据长度不足")
	}

	speedRaw := binary.BigEndian.Uint16(data[3:5])
	mileageRaw := binary.BigEndian.Uint32(data[5:9])
	voltRaw := binary.BigEndian.Uint16(data[9:11])
	currRaw := binary.BigEndian.Uint16(data[11:13])
	insRes := binary.BigEndian.Uint16(data[16:18])

	return &VehicleData{
		Status:       data[0],
		ChargeStatus: data[1],
		RunMode:      data[2],
		Speed:        float32(speedRaw) / 10.0,
		TotalMileage: float64(mileageRaw) / 10.0,
		Voltage:      float32(voltRaw) / 10.0,
		// 电流偏移量 1000A。即 0 表示 -1000A，10000(0x2710) 表示 0A。
		// 公式: 实值 = (原始值 * 0.1) - 1000.0
		Current:       (float32(currRaw) * 0.1) - 1000.0,
		SOC:           data[13],
		DCStatus:      data[14],
		Gear:          data[15],
		InsulationRes: insRes,
		AccelPedal:    data[18],
		BrakePedal:    data[19],
	}, nil
}
