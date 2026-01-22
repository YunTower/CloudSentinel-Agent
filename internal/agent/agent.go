package agent

import (
	"agent/config"
	"agent/internal/collector"
	"agent/internal/logger"
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

type Agent struct {
	cfg       config.Config
	logger    *logger.Logger
	sys       *system.System
	client    *websocket.Client
	collector *collector.Collector
	pm        *process.ProcessManager
	wg        sync.WaitGroup
	sigChan   chan os.Signal
	stopChan  chan struct{}
	mu        sync.Mutex
	running   bool
}

// NewAgent 创建新的Agent实例
func NewAgent(cfg config.Config) (*Agent, error) {
	// 设置时区
	timezone := cfg.Timezone
	if timezone == "" {
		timezone = "Asia/Shanghai" // 默认上海时区
	}
	location, err := time.LoadLocation(timezone)
	if err != nil {
		// 如果加载时区失败，尝试使用默认的上海时区
		location, err = time.LoadLocation("Asia/Shanghai")
		if err != nil {
			// 如果还是失败，使用 UTC
			location = time.UTC
		}
	}
	// 设置全局时区
	time.Local = location

	// 初始化日志
	logger := config.InitLogger(cfg.LogPath)

	// 初始化系统信息
	sys := config.InitSystem()

	// 创建WebSocket客户端
	client := websocket.NewClient(cfg.Server, logger)

	// 创建数据收集器
	col := collector.NewCollector(sys, logger, client,
		cfg.MetricsInterval, cfg.DetailInterval, cfg.SystemInterval)

	// 创建进程管理器
	pm := process.NewProcessManager(logger)
	pm.SetClient(client)
	pm.SetCollector(col)
	pm.SetHeartbeatInterval(time.Duration(cfg.HeartbeatInterval) * time.Second)

	return &Agent{
		cfg:       cfg,
		logger:    logger,
		sys:       sys,
		client:    client,
		collector: col,
		pm:        pm,
		sigChan:   make(chan os.Signal, 1),
		stopChan:  make(chan struct{}),
		running:   false,
	}, nil
}

// Start 启动agent
func (a *Agent) Start() error {
	a.mu.Lock()
	if a.running {
		a.mu.Unlock()
		return nil
	}
	a.running = true
	a.mu.Unlock()

	// 连接到服务器
	if err := a.client.ConnectWithRetry(); err != nil {
		a.logger.Error("连接失败: %v", err)
		return err
	}

	// 启动进程监控
	go a.pm.MonitorProcesses()

	// 定义回调函数
	callbacks := reporter.ReporterCallbacks{
		OnAuthSuccess: func() {
			a.logger.Info("认证成功，启动子进程...")
			// 启动心跳和数据上报进程
			a.pm.StartHeartbeatProcess()
			a.pm.StartReporterProcess()
		},
		OnDisconnect: func() {
			a.logger.Info("连接断开，停止子进程...")
			// 停止所有子进程，等待重连后由 OnAuthSuccess 重新启动
			a.pm.StopHeartbeat()
			a.pm.StopReporter()
		},
		OnReload: func() {
			a.logger.Info("收到配置重载请求，正在重载配置...")
			if err := a.Reload(); err != nil {
				a.logger.Error("配置重载失败: %v", err)
			} else {
				a.logger.Info("配置重载成功")
			}
		},
	}

	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		reporter.StartReporter(a.client, a.logger, a.cfg, callbacks)
	}()

	// 设置信号处理，优雅退出
	signal.Notify(a.sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)

	// 启动信号处理循环
	go a.handleSignals()

	return nil
}

// handleSignals 处理系统信号
func (a *Agent) handleSignals() {
	for {
		select {
		case sig := <-a.sigChan:
			switch sig {
			case syscall.SIGHUP:
				// 重载配置
				a.logger.Info("收到SIGHUP信号，重载配置...")
				if err := a.Reload(); err != nil {
					a.logger.Error("重载配置失败: %v", err)
				} else {
					a.logger.Info("配置重载成功")
				}
			case os.Interrupt, syscall.SIGTERM:
				// 优雅退出
				a.logger.Info("收到退出信号，正在关闭...")
				a.Stop()
				return
			}
		case <-a.stopChan:
			return
		}
	}
}

// Stop 停止agent
func (a *Agent) Stop() {
	a.mu.Lock()
	if !a.running {
		a.mu.Unlock()
		return
	}
	a.running = false
	close(a.stopChan)
	a.mu.Unlock()

	// 优雅关闭所有子进程
	a.pm.Shutdown()
	a.client.Close()

	// 等待所有 goroutine 完成
	done := make(chan struct{})
	go func() {
		a.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		a.logger.Info("所有进程已优雅退出")
	case <-time.After(10 * time.Second):
		a.logger.Warn("等待进程退出超时")
	}
}

// Reload 重载配置
func (a *Agent) Reload() error {
	// 重新加载配置
	configPath := config.GetConfigPath()
	newCfg, err := config.LoadConfigFromFile(configPath)
	if err != nil {
		return err
	}

	// 更新配置
	a.mu.Lock()
	oldCfg := a.cfg
	a.cfg = newCfg
	a.mu.Unlock()

	// 如果间隔参数有变化，更新收集器和进程管理器
	if oldCfg.MetricsInterval != newCfg.MetricsInterval ||
		oldCfg.DetailInterval != newCfg.DetailInterval ||
		oldCfg.SystemInterval != newCfg.SystemInterval {
		a.collector.UpdateIntervals(
			newCfg.MetricsInterval,
			newCfg.DetailInterval,
			newCfg.SystemInterval,
		)
	}

	if oldCfg.HeartbeatInterval != newCfg.HeartbeatInterval {
		a.pm.SetHeartbeatInterval(time.Duration(newCfg.HeartbeatInterval) * time.Second)
	}

	// 如果服务器地址或密钥变化，需要重新连接
	if oldCfg.Server != newCfg.Server || oldCfg.Key != newCfg.Key {
		a.logger.Info("检测到服务器地址或密钥变化，需要重启以应用新配置")
		// 注意：这里只更新配置，实际重连由上层决定是否重启
	}

	return nil
}

// Wait 等待agent退出
func (a *Agent) Wait() {
	a.wg.Wait()
}

// IsRunning 检查agent是否正在运行
func (a *Agent) IsRunning() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.running
}
