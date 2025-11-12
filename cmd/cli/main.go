package main

import (
	"agent/config"
	"agent/internal/collector"
	"agent/internal/process"
	"agent/internal/reporter"
	"agent/internal/system"
	"agent/internal/websocket"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

const agentVersion = "0.0.1"

func main() {
	// 初始化配置
	cfg := config.LoadConfig()

	// 初始化日志
	logger := config.InitLogger(cfg.LogPath)

	// 初始化系统信息
	var sys *system.System = config.InitSystem()

	// 创建WebSocket客户端
	client := websocket.NewClient(cfg.Server, logger)
	defer client.Close()

	// 连接到服务器
	if err := client.ConnectWithRetry(); err != nil {
		logger.Error("连接失败: %v", err)
		os.Exit(1)
	}

	// 创建数据收集器
	col := collector.NewCollector(sys, logger, client,
		cfg.MetricsInterval, cfg.DetailInterval, cfg.SystemInterval)

	// 创建进程管理器
	pm := process.NewProcessManager(logger)
	pm.SetClient(client)
	pm.SetCollector(col)
	pm.SetHeartbeatInterval(time.Duration(cfg.HeartbeatInterval) * time.Second)

	// 启动进程监控
	go pm.MonitorProcesses()

	// 定义回调函数
	callbacks := reporter.ReporterCallbacks{
		OnAuthSuccess: func() {
			logger.Info("认证成功，启动子进程...")
			// 启动心跳和数据上报进程
			pm.StartHeartbeatProcess()
			pm.StartReporterProcess()
		},
		OnDisconnect: func() {
			logger.Info("连接断开，停止子进程...")
			// 停止所有子进程，等待重连后由 OnAuthSuccess 重新启动
			pm.StopHeartbeat()
			pm.StopReporter()
		},
	}

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		reporter.StartReporter(client, logger, cfg, callbacks)
	}()

	// 设置信号处理，优雅退出
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// 等待退出信号
	<-sigChan
	logger.Info("收到退出信号，正在关闭...")

	// 优雅关闭所有子进程
	pm.Shutdown()
	client.Close()

	// 等待所有 goroutine 完成
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		logger.Info("所有进程已优雅退出")
	case <-sigChan:
		logger.Warn("收到第二次退出信号，强制退出")
		os.Exit(1)
	}
}
