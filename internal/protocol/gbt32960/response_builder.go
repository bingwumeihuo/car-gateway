package gbt32960

import (
	"time"
)

// BuildVehicleLoginResponse 构建车辆登入应答报文
// 响应格式: [Time 6] [Seq 2] [Result 1]
// Result: 0x01 成功, 0x02 错误
func BuildVehicleLoginResponse(vin string, success bool, requestTime []byte) []byte {
	// 1. Time (Echo Request Time)
	respData := make([]byte, 9)
	if len(requestTime) >= 6 {
		copy(respData[0:6], requestTime[:6])
	} else {
		// Fallback to now if missing
		now := time.Now()
		respData[0] = byte(now.Year() - 2000)
		respData[1] = byte(now.Month())
		respData[2] = byte(now.Day())
		respData[3] = byte(now.Hour())
		respData[4] = byte(now.Minute())
		respData[5] = byte(now.Second())
	}

	// Seq: 0 (Placeholder)

	// Result
	if success {
		respData[8] = 0x01 // Success
	} else {
		respData[8] = 0x02 // Fail
	}

	return respData
}

// BuildGeneralResponse 构建通用应答报文 (用于平台登入 0x05, 实时数据 0x02 等)
// 响应格式: [Time 6]
// 结果由 Header 中的 Response Flag 决定 (0x01 Success, 0x02 Fail)
func BuildGeneralResponse(requestTime []byte) []byte {
	respData := make([]byte, 6)
	if len(requestTime) >= 6 {
		copy(respData, requestTime[:6])
	} else {
		now := time.Now()
		respData[0] = byte(now.Year() - 2000)
		respData[1] = byte(now.Month())
		respData[2] = byte(now.Day())
		respData[3] = byte(now.Hour())
		respData[4] = byte(now.Minute())
		respData[5] = byte(now.Second())
	}
	return respData
}

// BuildLogoutResponse 构建登出应答 (复用车辆登入应答格式 [Time 6][Seq 2][Result 1])
func BuildLogoutResponse(vin string, success bool, requestTime []byte) []byte {
	return BuildVehicleLoginResponse(vin, success, requestTime)
}
