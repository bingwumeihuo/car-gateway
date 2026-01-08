package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"

	"vehicle-gateway/internal/config"
	"vehicle-gateway/internal/infra/rabbitmq"
	"vehicle-gateway/internal/server"
	"vehicle-gateway/internal/usecase"
	gbt32960 "vehicle-gateway/internal/usecase/gbt32960"
)

func main() {
	// 1. 配置加载
	cfg, err := config.LoadConfig("configs/config.yaml")
	if err != nil {
		panic(err)
	}

	// Init Logger
	writeSyncer := zapcore.AddSync(&lumberjack.Logger{
		Filename:   cfg.Log.Filename,
		MaxSize:    cfg.Log.MaxSize, // megabytes
		MaxBackups: cfg.Log.MaxBackups,
		MaxAge:     cfg.Log.MaxAge, // days
		Compress:   cfg.Log.Compress,
	})
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	// Parse Log Level
	level, err := zapcore.ParseLevel(cfg.Log.Level)
	if err != nil {
		level = zap.DebugLevel // Default
	}

	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig),
		writeSyncer,
		zap.NewAtomicLevelAt(level),
	)
	logger := zap.New(core, zap.AddCaller())
	defer logger.Sync()

	// 2. 基础设施层 (RabbitMQ & TSDB)
	producer, err := rabbitmq.NewRabbitMQProducer(cfg.RabbitMQ, logger)
	if err != nil {
		// With lazy connection, this error should be rare (only if config is invalid maybe)
		logger.Error("Failed to initialize RabbitMQ producer structure", zap.Error(err))
	} else {
		// We can use it even if not connected yet
		defer producer.Close()
	}

	// 3. 业务逻辑层 (分发器 & 处理器 & 会话管理)
	dispatcher := usecase.NewDataDispatcher(producer, 100, logger)
	dispatcher.Start()
	defer dispatcher.Stop()

	sm := gbt32960.NewSessionManager(logger)
	auth := gbt32960.NewInMemoryAuthService(cfg.Auth)
	h := gbt32960.NewHandler(sm, dispatcher, auth, logger) // Enable Dispatcher (RabbitMQ)

	// 4. 服务层
	srv := server.NewTCPServer(cfg, logger, h)

	// 5. 启动服务
	go func() {
		if err := srv.Start(context.Background()); err != nil {
			logger.Fatal("Server failed", zap.Error(err))
		}
	}()

	// 优雅停机
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down...")
	_ = srv.Stop(context.Background())
}
