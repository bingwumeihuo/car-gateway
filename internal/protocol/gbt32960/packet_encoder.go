package gbt32960

import (
	"encoding/binary"
)

// EncodePacket 将 Packet 结构体编码为字节流
func EncodePacket(pkt *Packet) []byte {
	// Structure: [Start 2][Cmd 1][Resp 1][VIN 17][Enc 1][Len 2][Data N][Check 1]
	// Header = 24 bytes
	dataLen := len(pkt.DataUnit)
	totalLen := 2 + 1 + 1 + 17 + 1 + 2 + dataLen + 1
	buf := make([]byte, totalLen)

	// 1. Start ##
	buf[0] = 0x23
	buf[1] = 0x23

	// 2. Cmd
	buf[2] = pkt.Command

	// 3. Response (New Field)
	if pkt.Response == 0 {
		buf[3] = 0xFE // Default to Command/Request
	} else {
		buf[3] = pkt.Response
	}

	// 4. VIN (17 chars)
	copy(buf[4:21], pkt.VIN)

	// 5. Enc
	buf[21] = pkt.Encryption

	// 6. Length
	binary.BigEndian.PutUint16(buf[22:24], uint16(dataLen))

	// 7. Data
	if dataLen > 0 {
		copy(buf[24:24+dataLen], pkt.DataUnit)
	}

	// 8. BCC Checksum
	// Calculate from Cmd(Index 2) to End of Data
	bcc := CalculateChecksum(buf[2 : 24+dataLen])
	buf[totalLen-1] = bcc

	return buf
}
