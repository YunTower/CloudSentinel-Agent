package main

import (
	"agent/config"
	"agent/internal/reporter"
	"agent/internal/websocket"
	"os"
	"os/signal"
	"syscall"
)

const agentVersion = "0.0.1"

func main() {
	// 初始化配置
	cfg := config.LoadConfig()

	// 初始化日志
	logger := config.InitLogger(cfg.LogPath)

	// 初始化系统信息
	system := config.InitSystem()

	// 创建WebSocket客户端
	client := websocket.NewClient(cfg.Server, logger)
	defer client.Close()

	// 连接到服务器（带重试）
	if err := client.ConnectWithRetry(); err != nil {
		logger.Error("连接失败: %v", err)
		os.Exit(1)
	}

	// 启动心跳
	go client.StartHeartbeat()

	// 设置信号处理，优雅退出
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	
	go func() {
		<-sigChan
		logger.Info("收到退出信号，正在关闭...")
		client.Close()
		os.Exit(0)
	}()

	// 启动消息处理和数据上报
	reporter.StartReporter(client, system, logger, cfg)
}
