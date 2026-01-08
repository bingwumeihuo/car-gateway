package gbt32960

import (
	"encoding/hex"
	"errors"
	"fmt"
	"vehicle-gateway/internal/protocol/gbt32960"
	"vehicle-gateway/internal/usecase"

	"runtime/debug"

	"go.uber.org/zap"
)

// Protocol Commands will be used from package gbt32960 directly

type Handler struct {
	SessionMgr *SessionManager
	Dispatcher *usecase.DataDispatcher
	Auth       AuthService
	logger     *zap.Logger
}

func NewHandler(sm *SessionManager, dispatcher *usecase.DataDispatcher, auth AuthService, logger *zap.Logger) *Handler {
	return &Handler{
		SessionMgr: sm,
		Dispatcher: dispatcher,
		Auth:       auth,
		logger:     logger,
	}
}

// HandleMessage 处理单个解析后的报文
func (h *Handler) HandleMessage(conn Conn, packet *gbt32960.Packet) (err error) {
	defer func() {
		if r := recover(); r != nil {
			stack := debug.Stack()
			h.logger.Error("Panic in HandleMessage",
				zap.Any("recover", r),
				zap.String("vin", packet.VIN),
				zap.String("stack", string(stack)))
			err = fmt.Errorf("internal server error: %v", r)
		}
	}()

	switch packet.Command {
	case gbt32960.CmdPlatformLogin:
		return h.handlePlatformLogin(conn, packet)
	case gbt32960.CmdVehicleLogin:
		return h.handleVehicleLogin(conn, packet)
	case gbt32960.CmdRealTime:
		return h.handleRealTime(conn, packet)
	case gbt32960.CmdLogout:
		return h.handleLogout(conn, packet)
	default:
		// 其他命令更新活跃时间
		h.SessionMgr.UpdateLastActive(packet.VIN)
		h.logger.Warn("Received unknown command",
			zap.String("vin", packet.VIN),
			zap.Uint8("command", packet.Command))
		return nil
	}
}

func (h *Handler) handlePlatformLogin(conn Conn, packet *gbt32960.Packet) error {
	loginData, err := gbt32960.ParsePlatformLogin(packet.DataUnit)
	if err != nil {
		return fmt.Errorf("平台登入解析失败: %v", err)
	}

	h.logger.Info("Platform Login Request",
		zap.String("username", loginData.Username),
		zap.String("raw_hex", hex.EncodeToString(packet.DataUnit)))

	// 认证校验
	success := true
	if h.Auth != nil {
		if err := h.Auth.PlatformLogin(loginData.Username, loginData.Password); err != nil {
			h.logger.Warn("Platform Auth failed",
				zap.String("username", loginData.Username),
				zap.Error(err))
			success = false
		}
	}

	// 构建响应 (复用 gbt32960.CmdPlatformLogin 作为 Command)
	// Response Flag: 0x01 (Success) / 0x02 (Fail)
	respFlag := byte(0x01)
	if !success {
		respFlag = 0x02
	}
	// Data Unit: [Time 6] Only (General Response)
	// Extract time from request (first 6 bytes)
	var reqTime []byte
	if len(packet.DataUnit) >= 6 {
		reqTime = packet.DataUnit[:6]
	}

	respData := gbt32960.BuildGeneralResponse(reqTime)
	respPkt := &gbt32960.Packet{
		Command:    gbt32960.CmdPlatformLogin,
		Response:   respFlag, // Set Header Response Flag
		VIN:        packet.VIN,
		Encryption: 0x01,
		DataUnit:   respData,
	}
	respBytes := gbt32960.EncodePacket(respPkt)

	if _, err := conn.Write(respBytes); err != nil {
		h.logger.Error("Failed to send platform login response", zap.Error(err))
	}

	if !success {
		return errors.New("平台鉴权失败，拒绝连接")
	}

	// Mark session as platform authenticated
	conn.SetPlatformAuthenticated(true)

	return nil
}

func (h *Handler) handleVehicleLogin(conn Conn, packet *gbt32960.Packet) error {
	// Extract time
	var reqTime []byte
	if len(packet.DataUnit) >= 6 {
		reqTime = packet.DataUnit[:6]
	}

	// Check Platform Authentication first
	if !conn.IsPlatformAuthenticated() {
		h.logger.Warn("Refused Vehicle Login: Platform not authenticated", zap.String("vin", packet.VIN))

		// Send Failure Response
		respData := gbt32960.BuildVehicleLoginResponse(packet.VIN, false, reqTime) // 0x02 Fail in Body + Header
		respPkt := &gbt32960.Packet{
			Command:    gbt32960.CmdVehicleLogin, // Reply to 0x01
			Response:   0x02,                     // Header Fail
			VIN:        packet.VIN,
			Encryption: 0x01,
			DataUnit:   respData,
		}
		if _, err := conn.Write(gbt32960.EncodePacket(respPkt)); err != nil {
			h.logger.Error("Failed to send login reject response", zap.Error(err))
		}

		return errors.New("请先进行平台登入")
	}

	loginData, err := gbt32960.ParseLogin(packet.DataUnit)
	if err != nil {
		return fmt.Errorf("车辆登入解析失败: %v", err)
	}

	h.logger.Info("Vehicle Login Request",
		zap.String("vin", packet.VIN),
		zap.String("collect_time", fmt.Sprintf("%v", loginData.CollectTime)),
		zap.String("password", loginData.Password))

	// 认证校验
	success := true

	// 构建响应
	respFlag := byte(0x01)
	if !success {
		respFlag = 0x02
	}

	respData := gbt32960.BuildVehicleLoginResponse(packet.VIN, success, reqTime)
	respPkt := &gbt32960.Packet{
		Command:    gbt32960.CmdVehicleLogin, // 0x01 Reply
		Response:   respFlag,                 // Set Header Response Flag
		VIN:        packet.VIN,
		Encryption: 0x01,
		DataUnit:   respData,
	}
	respBytes := gbt32960.EncodePacket(respPkt)

	if _, err := conn.Write(respBytes); err != nil {
		h.logger.Error("Failed to send vehicle login response", zap.Error(err))
	}

	if !success {
		return errors.New("车辆鉴权失败")
	}

	h.SessionMgr.Add(packet.VIN, conn)
	return nil
}

func (h *Handler) handleLogout(conn Conn, packet *gbt32960.Packet) error {
	logoutData, err := gbt32960.ParseLogout(packet.DataUnit)
	if err != nil {
		return fmt.Errorf("登出解析失败: %v", err)
	}
	h.logger.Info("Logout Request", zap.String("vin", packet.VIN), zap.Uint16("seq", logoutData.LogoutSeq))
	h.SessionMgr.Remove(packet.VIN)

	// Send Response (If explicitly requested or always? Standard implies response)
	// Response: [Time 6][Seq 2][Result 1]
	// Extract time
	var reqTime []byte
	if len(packet.DataUnit) >= 6 {
		reqTime = packet.DataUnit[:6]
	}
	respData := gbt32960.BuildLogoutResponse(packet.VIN, true, reqTime)
	respPkt := &gbt32960.Packet{
		Command:    gbt32960.CmdLogout,
		Response:   0x01, // Success
		VIN:        packet.VIN,
		Encryption: 0x01,
		DataUnit:   respData,
	}
	if _, err := conn.Write(gbt32960.EncodePacket(respPkt)); err != nil {
		h.logger.Error("Failed to send logout response", zap.Error(err))
	}

	return nil
}

func (h *Handler) handleRealTime(conn Conn, packet *gbt32960.Packet) error {
	// Logger Optimization
	logger := h.logger.With(zap.String("vin", packet.VIN))

	if _, ok := h.SessionMgr.Get(packet.VIN); !ok {
		// Auto-register session if missing (No Auth required)
		h.SessionMgr.Add(packet.VIN, conn)
	}
	h.SessionMgr.UpdateLastActive(packet.VIN)

	h.logger.Info("Received Real Time Data", zap.String("vin", packet.VIN))

	// 数据单元格式: [采集时间 6Byte] [信息类型 1Byte][信息体] [信息类型 1Byte][信息体] ...
	data := packet.DataUnit
	if len(data) < 6 {
		return errors.New("实时数据时间字段缺失")
	}

	// Extract time for response
	reqTime := data[:6]

	rest := data[6:]

	for len(rest) > 0 {
		infoType := gbt32960.RealTimeDataType(rest[0])
		rest = rest[1:]

		var processedBytes int
		var err error

		switch infoType {
		case gbt32960.DataTypeVehicle: // 0x01 整车数据 (20字节)
			if len(rest) < 20 {
				return errors.New("整车数据不足")
			}
			vd, err := gbt32960.ParseVehicleData(rest[:20])
			if err != nil {
				return err
			}
			logger.Debug("Vehicle Data", zap.Any("data", vd))

			if h.Dispatcher != nil {
				h.Dispatcher.Dispatch(usecase.MQPayload{Type: "VEHICLE", VIN: packet.VIN, Data: vd})
			}
			processedBytes = 20

		case gbt32960.DataTypeMotor: // 0x02 驱动电机
			md, err := gbt32960.ParseMotorData(rest)
			if err != nil {
				return err
			}
			processedBytes = 1 + int(md.Count)*12
			logger.Debug("Motor Data", zap.Any("data", md))

			if h.Dispatcher != nil {
				h.Dispatcher.Dispatch(usecase.MQPayload{Type: "MOTOR", VIN: packet.VIN, Data: md})
			}

		case gbt32960.DataTypeFuelCell: // 0x03 燃料电池
			fd, err := gbt32960.ParseFuelCellData(rest)
			if err != nil {
				return err
			}
			processedBytes = 8 + int(fd.TempProbeCount)
			logger.Debug("Fuel Cell Data", zap.Any("data", fd))

			if h.Dispatcher != nil {
				h.Dispatcher.Dispatch(usecase.MQPayload{Type: "FUEL_CELL", VIN: packet.VIN, Data: fd})
			}

		case gbt32960.DataTypeEngine: // 0x04 发动机
			if len(rest) < 5 {
				return errors.New("发动机数据不足")
			}
			ed, err := gbt32960.ParseEngineData(rest[:5])
			if err != nil {
				return err
			}
			logger.Debug("Engine Data", zap.Any("data", ed))
			if h.Dispatcher != nil {
				h.Dispatcher.Dispatch(usecase.MQPayload{Type: "ENGINE", VIN: packet.VIN, Data: ed})
			}
			processedBytes = 5

		case gbt32960.DataTypeLocation: // 0x05 车辆位置数据
			if len(rest) < 9 {
				return errors.New("车辆位置数据不足")
			}
			ld, err := gbt32960.ParseLocationData(rest[:9])
			if err != nil {
				return err
			}
			logger.Debug("Location Data", zap.Any("data", ld))
			if h.Dispatcher != nil {
				h.Dispatcher.Dispatch(usecase.MQPayload{Type: "LOCATION", VIN: packet.VIN, Data: ld})
			}
			processedBytes = 9

		case 0x06: // 2016: Extreme, 2025: Alarm
			if packet.Version == gbt32960.Version2025 {
				// 2025 Alarm (N1-N5)
				ad, err := gbt32960.ParseAlarmData2025(rest)
				if err != nil {
					logger.Warn("Alarm Data (2025) Parse Failed", zap.Error(err), zap.String("hex", hex.EncodeToString(rest)))
					return err
				}
				// 2025 Alarm has N5
				// Note: ad.*Faults are counts.
				sz := 5 +
					1 + 4*int(ad.BatteryFaults) +
					1 + 4*int(ad.MotorFaults) +
					1 + 4*int(ad.EngineFaults) +
					1 + 4*int(ad.OtherFaults) +
					1 + 2*int(ad.GeneralFaults)
				processedBytes = sz

				logger.Debug("Alarm Data (2025)", zap.Any("data", ad))
				if h.Dispatcher != nil {
					h.Dispatcher.Dispatch(usecase.MQPayload{Type: "ALARM", VIN: packet.VIN, Data: ad})
				}
			} else {
				// 2016 Extreme
				if len(rest) < 14 {
					return errors.New("极值数据不足")
				}
				xd, err := gbt32960.ParseExtremeData(rest[:14])
				if err != nil {
					return err
				}
				processedBytes = 14
				logger.Debug("Extreme Data (2016)", zap.Any("data", xd))
				if h.Dispatcher != nil {
					h.Dispatcher.Dispatch(usecase.MQPayload{Type: "EXTREME", VIN: packet.VIN, Data: xd})
				}
			}

		case 0x07: // 2016: Alarm, 2025: Battery Voltage
			if packet.Version == gbt32960.Version2025 {
				// 2025 Battery Voltage
				bd, err := gbt32960.ParseBatteryVoltageData2025(rest)
				if err != nil {
					return err
				}
				// Calc bytes
				// Header 1
				pBytes := 1
				for _, p := range bd.PackVoltages {
					pBytes += 7 + int(p.SingleCellCount)*2
				}
				processedBytes = pBytes
				logger.Debug("Battery Voltage (2025)", zap.Any("data", bd))
				if h.Dispatcher != nil {
					h.Dispatcher.Dispatch(usecase.MQPayload{Type: "BATTERY_VOLTAGE", VIN: packet.VIN, Data: bd})
				}
			} else {
				// 2016 Alarm
				ad, err := gbt32960.ParseAlarmData2016(rest)
				if err != nil {
					logger.Warn("Alarm Data (2016) Parse Failed", zap.Error(err), zap.String("hex", hex.EncodeToString(rest)))
					return err
				}
				sz := 5 +
					1 + 4*int(ad.BatteryFaults) +
					1 + 4*int(ad.MotorFaults) +
					1 + 4*int(ad.EngineFaults) +
					1 + 4*int(ad.OtherFaults)
				processedBytes = sz
				logger.Debug("Alarm Data (2016)", zap.Any("data", ad))
				if h.Dispatcher != nil {
					h.Dispatcher.Dispatch(usecase.MQPayload{Type: "ALARM", VIN: packet.VIN, Data: ad})
				}
			}

		case 0x08: // 2016: Storage Voltage, 2025: Battery Temp
			if packet.Version == gbt32960.Version2025 {
				// 2025 Battery Temp
				bt, err := gbt32960.ParseBatteryTempData2025(rest)
				if err != nil {
					return err
				}
				pBytes := 1
				for _, p := range bt.PackTemps {
					pBytes += 3 + int(p.ProbeCount)
				}
				processedBytes = pBytes
				logger.Debug("Battery Temp (2025)", zap.Any("data", bt))
				if h.Dispatcher != nil {
					h.Dispatcher.Dispatch(usecase.MQPayload{Type: "BATTERY_TEMP", VIN: packet.VIN, Data: bt})
				}
			} else {
				// 2016 Storage Voltage
				sv, err := gbt32960.ParseStorageVoltageData2016(rest)
				if err != nil {
					return err
				}
				// Calc bytes
				pBytes := 1
				for _, s := range sv.Subsystems {
					pBytes += 10 + int(s.FrameCellCount)*2
				}
				processedBytes = pBytes
				logger.Debug("Storage Voltage (2016)", zap.Any("data", sv))
				if h.Dispatcher != nil {
					h.Dispatcher.Dispatch(usecase.MQPayload{Type: "STORAGE_VOLTAGE", VIN: packet.VIN, Data: sv})
				}
			}

		case 0x09: // 2016: Storage Temp, 2025: Custom Start
			if packet.Version == gbt32960.Version2025 {
				// 2025 Custom
				processedBytes = len(rest)
				logger.Warn("Received Custom Data 0x09 (2025), consuming remaining", zap.Int("len", processedBytes))
			} else {
				// 2016 Storage Temp
				st, err := gbt32960.ParseStorageTempData2016(rest)
				if err != nil {
					return err
				}
				pBytes := 1
				for _, s := range st.Subsystems {
					pBytes += 3 + int(s.ProbeCount)
				}
				processedBytes = pBytes
				logger.Debug("Storage Temp (2016)", zap.Any("data", st))
				if h.Dispatcher != nil {
					h.Dispatcher.Dispatch(usecase.MQPayload{Type: "STORAGE_TEMP", VIN: packet.VIN, Data: st})
				}
			}

		// Extensions (Assuming both support 0x30+, or just 2025.
		// Since 2016 doesn't define 0x30, safe to support it if sent.)
		case 0x30:
			// Fuel Cell Stack
			fc, err := gbt32960.ParseFuelCellStackData(rest)
			if err != nil {
				return err
			}
			pBytes := 1
			for _, s := range fc.Stacks {
				pBytes += 10 + int(s.ProbeCount) // Temp probe only?
			}
			processedBytes = pBytes
			logger.Debug("Fuel Cell Stack", zap.Any("data", fc))
			if h.Dispatcher != nil {
				h.Dispatcher.Dispatch(usecase.MQPayload{Type: "FUEL_CELL_STACK", VIN: packet.VIN, Data: fc})
			}

		case 0x31:
			// Super Cap
			sc, err := gbt32960.ParseSuperCapData(rest)
			if err != nil {
				return err
			}
			processedBytes = 7 + int(sc.SingleCellCount)*2 + 2 + int(sc.ProbeCount)
			logger.Debug("Super Cap", zap.Any("data", sc))
			if h.Dispatcher != nil {
				h.Dispatcher.Dispatch(usecase.MQPayload{Type: "SUPER_CAP", VIN: packet.VIN, Data: sc})
			}

		case 0x32:
			// Super Cap Extreme
			sce, err := gbt32960.ParseSuperCapExtremeData(rest)
			if err != nil {
				return err
			}
			processedBytes = 18
			logger.Debug("Super Cap Extreme", zap.Any("data", sce))
			if h.Dispatcher != nil {
				h.Dispatcher.Dispatch(usecase.MQPayload{Type: "SUPER_CAP_EXTREME", VIN: packet.VIN, Data: sce})
			}

		default:
			logger.Warn("Unknown info type, stopping parse", zap.Uint8("type", uint8(infoType)))
			goto EndParse
		}

		if err != nil {
			return err
		}

		if len(rest) < processedBytes {
			return errors.New("数据解析溢出")
		}
		rest = rest[processedBytes:]
	}

EndParse:
	// Send General Response if requested (0xFE)
	if packet.Response == 0xFE {
		respData := gbt32960.BuildGeneralResponse(reqTime)
		respPkt := &gbt32960.Packet{
			Command:    gbt32960.CmdRealTime,
			Response:   0x01, // Success
			VIN:        packet.VIN,
			Encryption: 0x01,
			DataUnit:   respData,
		}
		if _, err := conn.Write(gbt32960.EncodePacket(respPkt)); err != nil {
			logger.Error("Failed to send realtime response", zap.Error(err))
		}
	}

	return nil
}
