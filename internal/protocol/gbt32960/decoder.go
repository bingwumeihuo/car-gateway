package gbt32960

import (
	"bytes"
	"encoding/binary"
	"errors"
)

var (
	// ErrTooLarge 当报文过大时返回 (安全检查)
	ErrTooLarge = errors.New("报文过大")
	// ErrInvalidHeader 当头部解析失败时返回 (应跳过处理)
	ErrInvalidHeader = errors.New("无效头部")
)

// PacketScanner 为 bufio.Scanner 提供 Split 函数
type PacketScanner struct {
	maxPacketSize int
}

// NewPacketScanner 创建一个新的扫描器助手。
// maxPacketSize 限制详细报文大小以防止恶意数据导致的 OOM (例如 64KB 或更大)。
func NewPacketScanner(maxPacketSize int) *PacketScanner {
	return &PacketScanner{maxPacketSize: maxPacketSize}
}

// SplitFunc 是用于 bufio.Scanner 解析 GB/T 32960 帧的分割函数。
func (ps *PacketScanner) SplitFunc(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}

	// 1. 搜索起始符 "##" (0x23 23) 或 "$$" (0x24 24)
	idx23 := bytes.Index(data, []byte{0x23, 0x23})
	idx24 := bytes.Index(data, []byte{0x24, 0x24})

	var startIdx int
	// Determine which one appears first (valid start)
	if idx23 != -1 && idx24 != -1 {
		if idx23 < idx24 {
			startIdx = idx23
		} else {
			startIdx = idx24
		}
	} else if idx23 != -1 {
		startIdx = idx23
	} else if idx24 != -1 {
		startIdx = idx24
	} else {
		// Not found
		if atEOF {
			return len(data), nil, nil
		}
		// Request more, discard useless except last one byte (in case it is half header)
		return len(data) - 1, nil, nil
	}

	// 如果在起始符之前发现垃圾数据，跳过它们，移动到起始符
	if startIdx > 0 {
		return startIdx, nil, nil
	}

	// Check header correctness (Start chars)
	// data[0] could be 0x23 or 0x24

	// 此时 data[0] == 0x23, data[1] == 0x23
	// 2. 检查是否有足够的字节解析头部 (以读取长度)
	// 读取长度需要到第 22,23 字节。
	// 头部固定部分总长: 2 + 1 + 1 + 17 + 1 + 2 = 24 字节。
	const HeaderFixedSize = 24
	if len(data) < HeaderFixedSize {
		// 需要更多数据
		if atEOF {
			// EOF 时头部不完整
			return len(data), nil, nil
		}
		return 0, nil, nil
	}

	// 3. 解析数据单元长度
	// 长度位于索引 22 (高位) 和 23 (低位)
	dataLen := binary.BigEndian.Uint16(data[22:24])

	// 总报文长度 = 固定头部(23) + 数据长度 + 校验码(1)
	totalLen := HeaderFixedSize + int(dataLen) + 1

	if totalLen > ps.maxPacketSize {
		// 报文声明长度过大。这可能是看起来像头部的垃圾数据。
		// 我们应该跳过当前的 "##" 并尝试寻找下一个，以避免死锁。
		// 前进 2 字节跳过当前的 "##"
		return 2, nil, nil
	}

	// 4. 检查是否拥有完整的报文
	if len(data) < totalLen {
		if atEOF {
			return len(data), nil, nil // EOF 时不完整，丢弃
		}
		return 0, nil, nil // 请求更多数据
	}

	// 5. 我们拥有完整的一帧。将其作为 token 返回。

	if !VerifyChecksum(data[:totalLen]) {
		// 校验失败。这个起始符可能是巧合。
		// 跳过第一个起始符并继续搜索。
		return 2, nil, nil
	}

	// 有效报文
	return totalLen, data[:totalLen], nil
}
