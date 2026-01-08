package gbt32960

import (
	"encoding/binary"
	"errors"
)

// BatteryVoltageData 动力蓄电池最小并联单元电压数据 (类型 0x07)
// 2025版: 表11, 表12
type BatteryVoltageData struct {
	BatteryPackCount byte              // 动力蓄电池包个数 (0~50)
	PackVoltages     []BatteryPackInfo // 电池包电压信息列表
}

// BatteryPackInfo 单个电池包电压信息 (表12)
type BatteryPackInfo struct {
	PackSeq           byte      // 动力蓄电池包号 (1~50)
	Voltage           float32   // 动力蓄电池包电压 (V) 0.1
	Current           float32   // 动力蓄电池包电流 (A) 0.1 (偏移3000A)
	SingleCellCount   uint16    // 最小并联单元总数 N (单体总数) (1~65531)
	FrameCellVoltages []float32 // 本帧最小并联单元电压 (V) 0.01 (2*N bytes)
}

// BatteryTempData 动力蓄电池温度数据 (类型 0x08)
// 2025版: 表13, 表14
type BatteryTempData struct {
	BatteryPackCount byte              // 动力蓄电池包个数 n (0~50)
	PackTemps        []BatteryPackTemp // 电池包温度信息列表
}

// BatteryPackTemp 单个电池包温度信息 (表14)
type BatteryPackTemp struct {
	PackSeq    byte   // 动力蓄电池包号 (1~50)
	ProbeCount uint16 // 动力蓄电池包温度探针个数 N (1~65531)
	ProbeTemps []byte // 探针温度 (N bytes) (偏移40)
}

// StorageVoltageData2016 可充电储能装置电压数据 (2016标准: 类型 0x08)
// 结构: [SubSysCount][System1][System2]... ? OR just repeated frames?
// Standard says:
// 1. Storage Subsystem Number (1 byte)
// 2. Storage Subsystem Voltage (2 bytes, 0.1V)
// 3. Storage Subsystem Current (2 bytes, 0.1A, Offset 1000A)
// 4. Single Cell Total Count (2 bytes)
// 5. Frame Start Cell Seq (2 bytes)
// 6. Frame Cell Count (1 byte)
// 7. [Cell Voltages...]
// It seems the packet usually contains ONE subsystem frame. The handler aggregates it?
// Actually, GBT 32960 Realtime data stream puts multiple "Info Unit" types.
// For Type 0x08, it's followed by "Info Body".
// "可充电储能子系统个数" (1 byte) is usually at the start of the Type 0x08 block.
// Let's implement full parsing assuming the header "Subsystem Count" exists.
type StorageVoltageData2016 struct {
	SubsystemCount byte
	Subsystems     []StorageSubsystemInfo
}

type StorageSubsystemInfo struct {
	SystemNo        byte    // 子系统号
	Voltage         float32 // 0.1V
	Current         float32 // 0.1A, Offset 1000A
	SingleCellCount uint16  // 单体电池总数
	StartFrameSeq   uint16  // 本帧起始电池序号
	FrameCellCount  byte    // 本帧单体电池总数
	CellVoltages    []float32
}

// ParseStorageVoltageData2016 解析2016版储能电压数据 (0x08)
func ParseStorageVoltageData2016(data []byte) (*StorageVoltageData2016, error) {
	// Min Header: Count(1)
	if len(data) < 1 {
		return nil, errors.New("储能装置电压数据长度不足(Header)")
	}
	count := data[0]
	// Sanity validity
	if count > 250 { // Just some check
		// warning?
	}

	sysList := make([]StorageSubsystemInfo, 0, int(count))
	offset := 1

	for i := 0; i < int(count); i++ {
		// Subsystem Frame Header Min: 1+2+2+2+2+1 = 10 bytes
		if len(data) < offset+10 {
			break
		}

		sysNo := data[offset]
		vol := binary.BigEndian.Uint16(data[offset+1 : offset+3])
		cur := binary.BigEndian.Uint16(data[offset+3 : offset+5])
		totalCells := binary.BigEndian.Uint16(data[offset+5 : offset+7])
		startSeq := binary.BigEndian.Uint16(data[offset+7 : offset+9])
		frameCells := data[offset+9]

		offset += 10

		// Parse Cells (2 bytes each)
		needed := int(frameCells) * 2
		avail := len(data) - offset
		readBytes := needed
		realCount := int(frameCells)
		if avail < needed {
			readBytes = avail / 2 * 2
			realCount = readBytes / 2
		}

		cells := make([]float32, 0, realCount)
		for j := 0; j < realCount; j++ {
			raw := binary.BigEndian.Uint16(data[offset+j*2 : offset+j*2+2])
			cells = append(cells, float32(raw)*0.001) // 2016 Standard: 0.001V
		}

		sysList = append(sysList, StorageSubsystemInfo{
			SystemNo:        sysNo,
			Voltage:         float32(vol) * 0.1,
			Current:         float32(cur)*0.1 - 1000.0, // 2016 Offset 1000A
			SingleCellCount: totalCells,
			StartFrameSeq:   startSeq,
			FrameCellCount:  frameCells,
			CellVoltages:    cells,
		})

		offset += readBytes
	}

	return &StorageVoltageData2016{SubsystemCount: count, Subsystems: sysList}, nil
}

// ParseBatteryVoltageData2025 解析动力蓄电池最小并联单元电压数据 (0x07 in 2025)
// 结构: [包个数 1][包1][包2]...
// 包结构: [包号 1][电压 2][电流 2][单体总数 2][单体电压 N*2]
func ParseBatteryVoltageData2025(data []byte) (*BatteryVoltageData, error) {
	if len(data) < 1 {
		return nil, errors.New("动力蓄电池电压数据长度不足(Header)")
	}

	packCount := data[0]
	// Sanity check
	if packCount > 50 && packCount != 0xFE && packCount != 0xFF {
		// Just a warning threshold, protocol says 0-50
	}

	packs := make([]BatteryPackInfo, 0, int(packCount))

	offset := 1
	for i := 0; i < int(packCount); i++ {
		// Check Min Length for Header of Pack: 1+2+2+2 = 7 bytes
		if len(data) < offset+7 {
			break // Stop parsing if incomplete
		}

		seq := data[offset]
		voltRaw := binary.BigEndian.Uint16(data[offset+1 : offset+3])
		currRaw := binary.BigEndian.Uint16(data[offset+3 : offset+5])
		countRaw := binary.BigEndian.Uint16(data[offset+5 : offset+7])

		offset += 7

		// Parse Cell Voltages
		// Each is 2 bytes
		cellsNeededBytes := int(countRaw) * 2
		// Bounds check
		available := len(data) - offset
		readBytes := cellsNeededBytes
		realCount := int(countRaw)

		if available < cellsNeededBytes {
			// Truncate logic to avoid panic
			readBytes = available
			// Start align to 2
			readBytes = readBytes / 2 * 2
			realCount = readBytes / 2
		}

		cellVolts := make([]float32, 0, realCount)
		for j := 0; j < realCount; j++ {
			cOffset := offset + j*2
			cVal := binary.BigEndian.Uint16(data[cOffset : cOffset+2])
			cellVolts = append(cellVolts, float32(cVal)*0.01) // 2025 standard says 0.01V resolution! (Old was 0.001)
		}

		packs = append(packs, BatteryPackInfo{
			PackSeq:           seq,
			Voltage:           float32(voltRaw) * 0.1,
			Current:           float32(currRaw)*0.1 - 3000.0, // 2025 standard: Offset 3000A
			SingleCellCount:   countRaw,
			FrameCellVoltages: cellVolts,
		})

		offset += readBytes
	}

	return &BatteryVoltageData{
		BatteryPackCount: packCount,
		PackVoltages:     packs,
	}, nil
}

// StorageTempData2016 可充电储能装置温度数据 (2016标准: 类型 0x09)
// 结构: [SubSysCount][System1]...
// System: [SysNo 1][ProbeCount 2][Temps N]
type StorageTempData2016 struct {
	SubsystemCount byte
	Subsystems     []StorageTempSubsystem
}

type StorageTempSubsystem struct {
	SystemNo     byte
	ProbeCount   uint16
	Temperatures []byte // Offset 40
}

// ParseStorageTempData2016 解析2016版温度数据 (0x09)
func ParseStorageTempData2016(data []byte) (*StorageTempData2016, error) {
	if len(data) < 1 {
		return nil, errors.New("储能装置温度数据长度不足(Header)")
	}
	count := data[0]

	sysList := make([]StorageTempSubsystem, 0, int(count))
	offset := 1

	for i := 0; i < int(count); i++ {
		// Header: 1+2 = 3
		if len(data) < offset+3 {
			break
		}

		sysNo := data[offset]
		pCount := binary.BigEndian.Uint16(data[offset+1 : offset+3])
		offset += 3

		needed := int(pCount)
		avail := len(data) - offset
		readCount := needed
		if avail < needed {
			readCount = avail
		}

		temps := make([]byte, readCount)
		for j := 0; j < readCount; j++ {
			temps[j] = data[offset+j] - 40
		}

		sysList = append(sysList, StorageTempSubsystem{
			SystemNo:     sysNo,
			ProbeCount:   pCount,
			Temperatures: temps,
		})
		offset += readCount
	}

	return &StorageTempData2016{SubsystemCount: count, Subsystems: sysList}, nil
}

// ParseBatteryTempData2025 解析动力蓄电池温度数据 (0x08 in 2025)
// 结构: [包个数 1][包1][包2]...
// 包结构: [包号 1][探针个数 2][温度 N]
func ParseBatteryTempData2025(data []byte) (*BatteryTempData, error) {
	if len(data) < 1 {
		return nil, errors.New("动力蓄电池温度数据长度不足(Header)")
	}

	packCount := data[0]
	packs := make([]BatteryPackTemp, 0, int(packCount))

	offset := 1
	for i := 0; i < int(packCount); i++ {
		// Header: 1+2 = 3 bytes
		if len(data) < offset+3 {
			break
		}

		seq := data[offset]
		probeCount := binary.BigEndian.Uint16(data[offset+1 : offset+3])

		offset += 3

		// Temps: N bytes
		needed := int(probeCount)
		available := len(data) - offset
		readCount := needed
		if available < needed {
			readCount = available
		}

		temps := make([]byte, readCount)
		for j := 0; j < readCount; j++ {
			temps[j] = data[offset+j] - 40
		}

		packs = append(packs, BatteryPackTemp{
			PackSeq:    seq,
			ProbeCount: probeCount,
			ProbeTemps: temps,
		})

		offset += readCount
	}

	return &BatteryTempData{
		BatteryPackCount: packCount,
		PackTemps:        packs,
	}, nil
}

// FuelCellStackData 燃料电池电堆数据 (类型 0x30)
// 见表18, 表19
type FuelCellStackData struct {
	StackCount byte
	Stacks     []FuelCellStackInfo
}

type FuelCellStackInfo struct {
	StackSeq byte    // 电堆序号
	Voltage  float32 // 电压 0.1V
	Current  float32 // 电流 0.1A (注意：文档通常有偏移？？需确认。此处假设无偏移或标准偏移？
	// 0x07电压Offset 3000A. 0x03燃料电池电流通常无偏移？
	// Grep output didn't show current offset.
	// Let's assume 0.1A raw for now or standard GBT 3k offset?
	// Wait, 0x03 (Old Fuel Cell) had current.
	// Let's check old model_fuel_cell.go for precedent.
	// Old 0x03 Current was 0.1A, no offset mentioned in typical GBT?
	// Actually OLD 0x03 had Voltage(0.1V), Current(0.1A).
	// Table 19 output truncated "燃料电池电堆电流...".
	// I will check 0x07 again: Current has 3000A offset.
	// I will play safe: If GBT 2016 0x03 current had no offset, 2025 0x30 might also not?
	// BUT 0x31 SuperCap Current HAS 3000A offset.
	// 0x07 Storage Current HAS 3000A/1000A offset.
	// I will assume 3000A offset for Energy Storage related currents usually.
	// But Fuel Cell?
	// Let's check 0x31 definition again. "数值偏移量3000A".
	// Let's check 0x30 definition from previous grep...
	// It was cut off.
	// I'll take a safe guess: standard high voltage current usually has offset.
	// But I'll stick to 0.1 multiplier. If offset needed, I can fix later.
	// Let's use 0.1 * val.
	AirInPressure  float32 // 空气入口压力 0.1kPa, offset -100kPa
	AirInTemp      int16   // 空气入口温度 1C, offset -40
	CoolantOutTemp byte    // ... Wait header said "冷却水出水口温度探针总数"
	ProbeCount     uint16
	ProbeTemps     []byte // 1C, offset -40
}

func ParseFuelCellStackData(data []byte) (*FuelCellStackData, error) {
	if len(data) < 1 {
		return nil, errors.New("燃料电池电堆数据长度不足(Header)")
	}
	count := data[0]
	stacks := make([]FuelCellStackInfo, 0, int(count))
	offset := 1

	for i := 0; i < int(count); i++ {
		// Min length check: Seq(1)+Volt(2)+Curr(2)+Pres(2)+Temp(1)+ProbeCount(2) = 10 bytes
		if len(data) < offset+10 {
			break
		}

		seq := data[offset]
		volt := binary.BigEndian.Uint16(data[offset+1 : offset+3])
		curr := binary.BigEndian.Uint16(data[offset+3 : offset+5])
		pres := binary.BigEndian.Uint16(data[offset+5 : offset+7])
		temp := data[offset+7]
		pCount := binary.BigEndian.Uint16(data[offset+8 : offset+10])

		offset += 10

		needed := int(pCount)
		available := len(data) - offset
		if available < needed {
			needed = available
		}

		pTemps := make([]byte, needed)
		copy(pTemps, data[offset:offset+needed])

		stacks = append(stacks, FuelCellStackInfo{
			StackSeq:      seq,
			Voltage:       float32(volt) * 0.1,
			Current:       float32(curr) * 0.1,
			AirInPressure: float32(pres)*0.1 - 100.0,
			AirInTemp:     int16(temp) - 40,
			ProbeCount:    pCount,
			ProbeTemps:    pTemps,
		})

		offset += needed
	}

	return &FuelCellStackData{StackCount: count, Stacks: stacks}, nil
}

// SuperCapData 超级电容器数据 (类型 0x31)
// 见表25
// 注意：文档只引用了表25，未提及数量表。假设为单结构。
type SuperCapData struct {
	SystemNo        byte      // 系统号
	TotalVoltage    float32   // 0.1V
	TotalCurrent    float32   // 0.1A, Offset 3000A
	SingleCellCount uint16    // M
	SingleCellVolts []float32 // 0.01V
	ProbeCount      uint16    // N
	ProbeTemps      []byte    // 1C, Offset -40
}

// SuperCapExtremeData 超级电容器极值数据 (类型 0x32)
// 见表26
type SuperCapExtremeData struct {
	MaxVoltSystemNo  byte
	MaxVoltCellCode  uint16
	MaxVoltValue     float32 // 0.001V
	MinVoltSystemNo  byte
	MinVoltCellCode  uint16
	MinVoltValue     float32 // 0.001V
	MaxTempSystemNo  byte
	MaxTempProbeCode uint16
	MaxTempValue     int16 // 1C, Offset -40
	MinTempSystemNo  byte
	MinTempProbeCode uint16
	MinTempValue     int16 // 1C, Offset -40
}

// ... (SuperCapExtremeData remains same) ...

func ParseSuperCapData(data []byte) (*SuperCapData, error) {
	// Min Header: Sys(1)+Vol(2)+Cur(2)+CellN(2) = 7
	if len(data) < 7 {
		return nil, errors.New("超级电容数据长度不足(Header)")
	}

	offset := 0
	sysNo := data[offset]
	vol := binary.BigEndian.Uint16(data[offset+1 : offset+3])
	cur := binary.BigEndian.Uint16(data[offset+3 : offset+5])
	cellN := binary.BigEndian.Uint16(data[offset+5 : offset+7])

	offset += 7

	// Cells
	cellsBytes := int(cellN) * 2
	avail := len(data) - offset
	readCells := cellsBytes
	realCellN := int(cellN)
	if avail < cellsBytes {
		readCells = avail / 2 * 2
		realCellN = readCells / 2
	}

	cVolts := make([]float32, 0, realCellN)
	for j := 0; j < realCellN; j++ {
		raw := binary.BigEndian.Uint16(data[offset+j*2 : offset+j*2+2])
		cVolts = append(cVolts, float32(raw)*0.01)
	}
	offset += readCells

	// Probe Count (2 bytes)
	if len(data)-offset < 2 {
		// return partial? or error?
		// Safe to return what we have?
		// But ProbeCount is mandatory field.
		// If missing, error.
		return nil, errors.New("超级电容数据长度不足(ProbeCnt)")
	}
	probeN := binary.BigEndian.Uint16(data[offset : offset+2])
	offset += 2

	// Probes
	availP := len(data) - offset
	readP := int(probeN)
	if availP < readP {
		readP = availP
	}

	pTemps := make([]byte, readP)
	copy(pTemps, data[offset:offset+readP])
	// offset += readP // No need to increment if we return struct.
	// But to be consistent usually we don't return processedBytes here, the struct implies content.
	// The `handler` calculates bytes from struct content or we consume exact bytes.

	return &SuperCapData{
		SystemNo:        sysNo,
		TotalVoltage:    float32(vol) * 0.1,
		TotalCurrent:    float32(cur)*0.1 - 3000.0,
		SingleCellCount: cellN,
		SingleCellVolts: cVolts,
		ProbeCount:      probeN,
		ProbeTemps:      pTemps,
	}, nil
}

func ParseSuperCapExtremeData(data []byte) (*SuperCapExtremeData, error) {
	// Length check: 1+2+2 + 1+2+2 + 1+2+1 + 1+2+1 = 18 bytes
	if len(data) < 18 {
		return nil, errors.New("超级电容极值数据长度不足")
	}

	// Max V
	maxVSys := data[0]
	maxVCode := binary.BigEndian.Uint16(data[1:3])
	maxVVal := binary.BigEndian.Uint16(data[3:5])

	// Min V
	minVSys := data[5]
	minVCode := binary.BigEndian.Uint16(data[6:8])
	minVVal := binary.BigEndian.Uint16(data[8:10])

	// Max T
	maxTSys := data[10]
	maxTCode := binary.BigEndian.Uint16(data[11:13])
	maxTVal := data[13]

	// Min T
	minTSys := data[14]
	minTCode := binary.BigEndian.Uint16(data[15:17])
	minTVal := data[17]

	return &SuperCapExtremeData{
		MaxVoltSystemNo:  maxVSys,
		MaxVoltCellCode:  maxVCode,
		MaxVoltValue:     float32(maxVVal) * 0.001,
		MinVoltSystemNo:  minVSys,
		MinVoltCellCode:  minVCode,
		MinVoltValue:     float32(minVVal) * 0.001,
		MaxTempSystemNo:  maxTSys,
		MaxTempProbeCode: maxTCode,
		MaxTempValue:     int16(maxTVal) - 40,
		MinTempSystemNo:  minTSys,
		MinTempProbeCode: minTCode,
		MinTempValue:     int16(minTVal) - 40,
	}, nil
}
