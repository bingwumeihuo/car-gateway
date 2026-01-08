package gbt32960

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// GB/T 32960.3-2025 协议常量定义
const (
	StartChar = 0x2323 // 起始符 "##"
	// HeaderLength: 2(Start) + 1(Cmd) + 1(Resp) + 17(VIN) + 1(Enc) + 2(Len) = 24
	HeaderLength = 24
	// MinPacketSize: Header + Checksum(1) = 25
	MinPacketSize = 25

	// 命令标识
	CmdVehicleLogin  = 0x01 // 车辆登入
	CmdPlatformLogin = 0x05 // 平台登入
	CmdRealTime      = 0x02
	CmdLogout        = 0x03
	CmdHeartbeat     = 0x07
)

type ProtocolVersion int

const (
	Version2016 ProtocolVersion = iota // 0x23 0x23 (##)
	Version2025                        // 0x24 0x24 ($$)
)

// Packet 代表一个解析后的 GB/T 32960 报文
type Packet struct {
	Version      ProtocolVersion // 协议版本
	Command      byte
	Response     byte // 应答标识: 0xFE=命令, 0x01=成功, 0x02=错误, 0x03=VIN重复
	VIN          string
	Username     string
	password     string
	Encryption   byte
	DataUnit     []byte
	OriginalData []byte // 原始字节数据
}

// ParseHeader 尝试从字节切片开头解析报文头。
// 返回预期的数据单元长度或错误。
// 假设切片以 "##" 开头。
func ParseHeader(data []byte) (dataLen uint16, err error) {
	if len(data) < HeaderLength { // 24
		return 0, errors.New("数据长度不足以解析头部")
	}

	// 检查起始符
	if data[0] == 0x23 && data[1] == 0x23 {
		// Version 2016
	} else if data[0] == 0x24 && data[1] == 0x24 {
		// Version 2025
	} else {
		return 0, fmt.Errorf("无效的起始符: %X%X", data[0], data[1])
	}

	// 数据单元长度位于索引 22, 23 (大端序)
	// 0-1: ##
	// 2: 命令单元
	// 3: 应答标识 (Response)
	// 4-20: 唯一识别码 (VIN)
	// 21: 加密方式
	// 22-23: 数据单元长度
	dataLen = binary.BigEndian.Uint16(data[22:24])
	return dataLen, nil
}

// VerifyChecksum 验证完整报文 packetData 的 BCC 校验码。
// Checksum Range: From Command (Index 2) to Data End.
func VerifyChecksum(packetData []byte) bool {
	if len(packetData) < MinPacketSize {
		return false
	}

	// 校验码是最后一个字节
	receivedBCC := packetData[len(packetData)-1]

	// 计算 BCC: 从命令单元 (索引 2) 到 数据单元 (切片末尾 - 2)
	// 报文结构: [起始 2][命令 1][应答 1][VIN 17][加密 1][长度 2][数据 N][校验 1]
	// 异或范围: 命令(2) ... 数据(Last)
	calculatedBCC := CalculateChecksum(packetData[2 : len(packetData)-1])

	return receivedBCC == calculatedBCC
}

// CalculateChecksum 计算给定数据的异或校验和 (XOR Checksum)
func CalculateChecksum(data []byte) byte {
	var bcc byte
	for _, b := range data {
		bcc ^= b
	}
	return bcc
}
