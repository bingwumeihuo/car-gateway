package main

import (
	"fmt"
	"net"
	"os"
	"time"
	"vehicle-gateway/internal/client"
)

const (
	ServerAddr   = "127.0.0.1:32960"
	PlatformUser = "admin"
	PlatformPass = "password_placeholder"

	VehicleVIN   = "VIN12345678901234"
	VehicleICCID = "12345678901234567890"
)

func main() {
	fmt.Println("启动测试客户端...")
	conn, err := net.Dial("tcp", ServerAddr)
	if err != nil {
		fmt.Printf("连接服务器失败: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()
	fmt.Printf("已连接到服务器 %s\n", ServerAddr)

	builder := client.NewPacketBuilder(VehicleVIN)

	// ==========================================
	// 1. 平台登入 (0x05)
	// ==========================================
	fmt.Println(">> [1/3] 发送平台登入请求 (0x05)...")
	platPkt := builder.BuildPlatformLogin(PlatformUser, PlatformPass)
	fmt.Printf("   HEX: %X\n", platPkt)
	if _, err := conn.Write(platPkt); err != nil {
		panic(err)
	}

	// 读取平台登入响应
	resp := make([]byte, 1024)
	n, err := conn.Read(resp)
	if err != nil {
		panic(err)
	}
	fmt.Printf("<< 收到响应 (%d 字节)\n", n)
	// 解析平台登入响应
	//fmt.Printf("   解析结果: %+v\n", resp)

	if n > 0 && resp[2] == 0x05 { // Check Command Byte (Byte Index 2 usually? No, Header is [Start][Cmd][RespBit] or similar)
		fmt.Println("   平台登入响应接收完毕")
	}

	// ==========================================
	// 2. 车辆登入 (0x01)
	// ==========================================
	fmt.Println(">> [2/3] 发送车辆登入请求 (0x01)...")
	vehPkt := builder.BuildVehicleLogin(VehicleICCID)
	if _, err := conn.Write(vehPkt); err != nil {
		panic(err)
	}

	// 读取车辆登入响应
	n, err = conn.Read(resp)
	if err != nil {
		panic(err)
	}
	fmt.Printf("<< 收到响应 (%d 字节)\n", n)
	// Check result

	// ==========================================
	// 3. 发送实时数据
	// ==========================================
	fmt.Println(">> [3/3] 开始发送实时数据...")
	for i := 0; i < 5; i++ {
		fmt.Printf(">> 发送实时数据 [%d]...\n", i+1)
		rtPkt := builder.BuildRealTime(float32(60+i), byte(80-i))
		conn.Write(rtPkt)
		time.Sleep(1 * time.Second)
	}

	// 3. 登出
	fmt.Println(">> 发送登出请求...")
	logoutPkt := builder.BuildLogout(100)
	conn.Write(logoutPkt)

	fmt.Println("测试完成，关闭连接")
}
