package gbt32960

// RealTimeDataType 定义实时数据类型
// 0x01: 整车数据
// 0x02: 驱动电机数据
// 0x03: 燃料电池数据
// 0x04: 发动机数据
// 0x05: 车辆位置数据
// 0x06: 极值数据
// 0x07: 报警数据
// 0x08: 可充电储能装置电压数据
// 0x09: 可充电储能装置温度数据
type RealTimeDataType byte

const (
	DataTypeVehicle        RealTimeDataType = 0x01 // 整车数据
	DataTypeMotor          RealTimeDataType = 0x02 // 驱动电机数据
	DataTypeFuelCell       RealTimeDataType = 0x03 // 燃料电池发动机及车载氢系统数据
	DataTypeEngine         RealTimeDataType = 0x04 // 发动机数据
	DataTypeLocation       RealTimeDataType = 0x05 // 车辆位置数据
	DataTypeExtreme        RealTimeDataType = 0x06 // 极值数据
	DataTypeAlarm          RealTimeDataType = 0x07 // 报警数据
	DataTypeStorageVoltage RealTimeDataType = 0x08 // 可充电储能装置电压数据
	DataTypeStorageTemp    RealTimeDataType = 0x09 // 可充电储能装置温度数据

	// 2025 Extension Types
	DataTypeFuelCellStack   RealTimeDataType = 0x30 // 燃料电池电堆数据
	DataTypeSuperCap        RealTimeDataType = 0x31 // 超级电容数据
	DataTypeSuperCapExtreme RealTimeDataType = 0x32 // 超级电容极值数据
)
