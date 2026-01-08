package gbt32960

import (
	"encoding/binary"
	"errors"
)

// AlarmData 报警数据 (类型 0x07 for 2016, 0x06 for 2025)
type AlarmData struct {
	MaxAlarmLevel byte     // 最高报警等级
	AlarmMask     uint32   // 通用报警标志 (位掩码)
	BatteryFaults byte     // 可充电储能装置故障总数 N1
	BatteryCodes  []uint32 // 故障代码列表 (4字节/个)
	MotorFaults   byte     // 驱动电机故障总数 N2
	MotorCodes    []uint32 // 故障代码列表
	EngineFaults  byte     // 发动机故障总数 N3
	EngineCodes   []uint32 // 故障代码列表
	OtherFaults   byte     // 其他故障总数 N4
	OtherCodes    []uint32 // 故障代码列表

	// 2025 Extended Fields
	GeneralFaults byte     // 通用报警故障总数 N5
	GeneralCodes  []uint16 // 通用报警故障等级列表
}

// ParseAlarmData2016 解析2016版报警数据 (N1-N4)
func ParseAlarmData2016(data []byte) (*AlarmData, error) {
	return parseAlarmDataCommon(data, false)
}

// ParseAlarmData2025 解析2025版报警数据 (N1-N5)
func ParseAlarmData2025(data []byte) (*AlarmData, error) {
	return parseAlarmDataCommon(data, true)
}

func parseAlarmDataCommon(data []byte, is2025 bool) (*AlarmData, error) {

	alarm := &AlarmData{
		MaxAlarmLevel: data[0],
		AlarmMask:     binary.BigEndian.Uint32(data[1:5]),
	}
	offset := 5

	// 1. Battery N1
	if offset >= len(data) {
		return nil, errors.New("报警数据不完整(N1)")
	}
	alarm.BatteryFaults = data[offset]
	offset++
	codes, bytesRead := readCodes(data[offset:], int(alarm.BatteryFaults))
	if codes == nil {
		return nil, errors.New("报警数据(电池)解析溢出")
	}
	alarm.BatteryCodes = codes
	offset += bytesRead

	// 2. Motor N2
	if offset >= len(data) {
		return nil, errors.New("报警数据不完整(N2)")
	}
	alarm.MotorFaults = data[offset]
	offset++
	codes, bytesRead = readCodes(data[offset:], int(alarm.MotorFaults))
	if codes == nil {
		return nil, errors.New("报警数据(电机)解析溢出")
	}
	alarm.MotorCodes = codes
	offset += bytesRead

	// 3. Engine N3
	if offset >= len(data) {
		return nil, errors.New("报警数据不完整(N3)")
	}
	alarm.EngineFaults = data[offset]
	offset++
	codes, bytesRead = readCodes(data[offset:], int(alarm.EngineFaults))
	if codes == nil {
		return nil, errors.New("报警数据(发动机)解析溢出")
	}
	alarm.EngineCodes = codes
	offset += bytesRead

	// 4. Other N4
	if offset >= len(data) {
		return nil, errors.New("报警数据不完整(N4)")
	}
	alarm.OtherFaults = data[offset]
	offset++
	codes, bytesRead = readCodes(data[offset:], int(alarm.OtherFaults))
	if codes == nil {
		return nil, errors.New("报警数据(其他)解析溢出")
	}
	alarm.OtherCodes = codes
	offset += bytesRead

	// 5. General N5 (2025 Only)
	if is2025 {
		if offset < len(data) {
			alarm.GeneralFaults = data[offset]
			offset++
			genCodes, _ := readGeneralCodes(data[offset:], int(alarm.GeneralFaults))
			if genCodes == nil {
				return nil, errors.New("报警数据(通用)解析溢出")
			}
			alarm.GeneralCodes = genCodes
		}
	}

	return alarm, nil
}

func readCodes(data []byte, count int) ([]uint32, int) {
	length := count * 4
	if len(data) < length {
		return nil, 0
	}
	codes := make([]uint32, count)
	for i := 0; i < count; i++ {
		codes[i] = binary.BigEndian.Uint32(data[i*4 : i*4+4])
	}
	return codes, length
}

func readGeneralCodes(data []byte, count int) ([]uint16, int) {
	length := count * 2
	if len(data) < length {
		return nil, 0
	}
	codes := make([]uint16, count)
	for i := 0; i < count; i++ {
		codes[i] = binary.BigEndian.Uint16(data[i*2 : i*2+2])
	}
	return codes, length
}
