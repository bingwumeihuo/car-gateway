package client

import (
	"encoding/binary"
	"time"

	"vehicle-gateway/internal/protocol/gbt32960"
)

// PacketBuilder 帮助构建测试用的 GB/T 32960 报文
type PacketBuilder struct {
	VIN string
}

func NewPacketBuilder(vin string) *PacketBuilder {
	return &PacketBuilder{VIN: vin}
}

// BuildPlatformLogin 生成平台登入报文 (0x05)
func (pb *PacketBuilder) BuildPlatformLogin(username, password string) []byte {
	payload := make([]byte, 41)

	// 1. Time (Current Time)
	now := time.Now()
	payload[0] = byte(now.Year() - 2000)
	payload[1] = byte(now.Month())
	payload[2] = byte(now.Day())
	payload[3] = byte(now.Hour())
	payload[4] = byte(now.Minute())
	payload[5] = byte(now.Second())

	// 2. Seq (serial number)
	binary.BigEndian.PutUint16(payload[6:8], 1)

	// 3. Username (12 bytes, zero padded)
	copy(payload[8:20], username)

	// 4. Password (20 bytes, zero padded)
	copy(payload[20:40], password)

	// 5. Encryption Mode (0x01 = No Encryption?)
	payload[40] = 0x01

	pkt := &gbt32960.Packet{
		Command:    gbt32960.CmdPlatformLogin,
		VIN:        pb.VIN,
		Encryption: 0x01,
		DataUnit:   payload,
	}
	return gbt32960.EncodePacket(pkt)
}

// BuildVehicleLogin 生成车辆登入报文 (0x01)
func (pb *PacketBuilder) BuildVehicleLogin(iccid string) []byte {
	// 采集时间 6 + 登入流水号 2 + ICCID 20 + 子系统数 1 + 编码 1 = 30 字节
	data := make([]byte, 30)

	// 当前时间
	now := time.Now()
	data[0] = byte(now.Year() - 2000)
	data[1] = byte(now.Month())
	data[2] = byte(now.Day())
	data[3] = byte(now.Hour())
	data[4] = byte(now.Minute())
	data[5] = byte(now.Second())

	// 流水号 (1)
	binary.BigEndian.PutUint16(data[6:8], 1)

	// ICCID
	copy(data[8:28], iccid)

	// 子系统个数 (1)
	data[28] = 1
	// 编码方式 (1)
	data[29] = 1

	pkt := &gbt32960.Packet{
		Command:    gbt32960.CmdVehicleLogin,
		VIN:        pb.VIN,
		Encryption: 0x01,
		DataUnit:   data,
	}
	return gbt32960.EncodePacket(pkt)
}

// BuildRealTime 生成实时数据报文 (含整车、电机)
func (pb *PacketBuilder) BuildRealTime(speed float32, soc byte) []byte {
	// 1. 整车数据 (20 bytes)
	vd := make([]byte, 20)
	vd[0] = 0x01 // Status
	// 车速 (0.1 km/h)
	speedRaw := uint16(speed * 10)
	binary.BigEndian.PutUint16(vd[3:5], speedRaw)
	// 电流 0A (值 10000)
	binary.BigEndian.PutUint16(vd[11:13], 10000)
	vd[13] = soc

	// 2. 驱动电机数据 (1 + 12 = 13 bytes)
	md := make([]byte, 13)
	md[0] = 1    // 个数
	md[1] = 1    // 序号
	md[2] = 0x01 // 状态

	// 组装完整报文
	// 时间 (6 bytes)
	timeBytes := make([]byte, 6)
	now := time.Now()
	timeBytes[0] = byte(now.Year() - 2000) // ... simplified filling

	var fullData []byte
	fullData = append(fullData, timeBytes...)
	fullData = append(fullData, 0x01) // 类型: 整车
	fullData = append(fullData, vd...)
	fullData = append(fullData, 0x02) // 类型: 电机
	fullData = append(fullData, md...)

	pkt := &gbt32960.Packet{
		Command:    gbt32960.CmdRealTime,
		VIN:        pb.VIN,
		Encryption: 0x05,
		DataUnit:   fullData,
	}
	return gbt32960.EncodePacket(pkt)
}

// BuildLogout 生成登出报文
func (pb *PacketBuilder) BuildLogout(seq uint16) []byte {
	data := make([]byte, 8)
	// 时间
	now := time.Now()
	data[0] = byte(now.Year() - 2000)
	// ...
	binary.BigEndian.PutUint16(data[6:8], seq)

	pkt := &gbt32960.Packet{
		Command:    gbt32960.CmdLogout,
		VIN:        pb.VIN,
		Encryption: 0x05,
		DataUnit:   data,
	}
	return gbt32960.EncodePacket(pkt)
}
