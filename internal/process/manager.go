package process

import (
	"agent/internal/collector"
	"agent/internal/logger"
	"agent/internal/websocket"
	"context"
	"sync"
	"time"
)

// ProcessManager 管理所有子进程的生命周期
type ProcessManager struct {
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	logger *logger.Logger

	// 子进程状态通道
	heartbeatHealth chan bool
	reporterHealth  chan bool

	// 子进程控制
	heartbeatCtx    context.Context
	heartbeatCancel context.CancelFunc
	reporterCtx     context.Context
	reporterCancel  context.CancelFunc

	// 重启控制
	heartbeatRestartDelay time.Duration
	reporterRestartDelay  time.Duration
	maxRestartDelay       time.Duration

	// 互斥锁保护重启逻辑
	mu sync.Mutex

	// 子进程引用
	client    *websocket.Client
	collector *collector.Collector
}

// NewProcessManager 创建新的进程管理器
func NewProcessManager(logger *logger.Logger) *ProcessManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &ProcessManager{
		ctx:                   ctx,
		cancel:                cancel,
		logger:                logger,
		heartbeatHealth:       make(chan bool, 10),
		reporterHealth:        make(chan bool, 10),
		heartbeatRestartDelay: 1 * time.Second,
		reporterRestartDelay:  1 * time.Second,
		maxRestartDelay:       64 * time.Second,
	}
}

// SetClient 设置 WebSocket 客户端
func (pm *ProcessManager) SetClient(client *websocket.Client) {
	pm.client = client
}

// SetCollector 设置数据收集器
func (pm *ProcessManager) SetCollector(col *collector.Collector) {
	pm.collector = col
}

// StartHeartbeatProcess 启动心跳进程
func (pm *ProcessManager) StartHeartbeatProcess() {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	// 如果已有心跳进程在运行，先停止
	if pm.heartbeatCancel != nil {
		pm.heartbeatCancel()
	}

	// 创建新的 context
	pm.heartbeatCtx, pm.heartbeatCancel = context.WithCancel(pm.ctx)

	pm.wg.Add(1)
	go pm.runHeartbeatProcess()
}

// runHeartbeatProcess 运行心跳进程
func (pm *ProcessManager) runHeartbeatProcess() {
	defer pm.wg.Done()

	for {
		select {
		case <-pm.ctx.Done():
			return
		default:
		}

		// 检查连接状态
		if pm.client == nil || !pm.client.IsConnected {
			pm.logger.Warn("心跳进程：WebSocket 未连接，等待重连...")
			time.Sleep(pm.heartbeatRestartDelay)
			pm.exponentialBackoff(&pm.heartbeatRestartDelay)
			continue
		}

		// 重置重启延迟
		pm.heartbeatRestartDelay = 1 * time.Second

		// 运行心跳
		pm.logger.Info("心跳进程：启动")
		pm.client.StartHeartbeat(pm.heartbeatCtx, pm.heartbeatHealth)

		// 心跳进程退出，检查是否需要重启
		select {
		case <-pm.ctx.Done():
			return
		default:
			pm.logger.Warn("心跳进程：异常退出，准备重启（延迟 %v）", pm.heartbeatRestartDelay)
			time.Sleep(pm.heartbeatRestartDelay)
			pm.exponentialBackoff(&pm.heartbeatRestartDelay)
		}
	}
}

// StartReporterProcess 启动数据上报进程
func (pm *ProcessManager) StartReporterProcess() {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	// 如果已有上报进程在运行，先停止
	if pm.reporterCancel != nil {
		pm.reporterCancel()
	}

	// 创建新的 context
	pm.reporterCtx, pm.reporterCancel = context.WithCancel(pm.ctx)

	pm.wg.Add(1)
	go pm.runReporterProcess()
}

// runReporterProcess 运行数据上报进程（带自动重启）
func (pm *ProcessManager) runReporterProcess() {
	defer pm.wg.Done()

	for {
		select {
		case <-pm.ctx.Done():
			return
		default:
		}

		// 检查连接状态
		if pm.client == nil || !pm.client.IsConnected {
			pm.logger.Warn("数据上报进程：WebSocket 未连接，等待重连...")
			time.Sleep(pm.reporterRestartDelay)
			pm.exponentialBackoff(&pm.reporterRestartDelay)
			continue
		}

		// 重置重启延迟
		pm.reporterRestartDelay = 1 * time.Second

		// 运行数据上报
		pm.logger.Info("数据上报进程：启动")
		pm.collector.StartPeriodicReporting(pm.reporterCtx, pm.reporterHealth)

		// 上报进程退出，检查是否需要重启
		select {
		case <-pm.ctx.Done():
			return
		default:
			pm.logger.Warn("数据上报进程：异常退出，准备重启（延迟 %v）", pm.reporterRestartDelay)
			time.Sleep(pm.reporterRestartDelay)
			pm.exponentialBackoff(&pm.reporterRestartDelay)
		}
	}
}

// MonitorProcesses 监控所有子进程的健康状态
func (pm *ProcessManager) MonitorProcesses() {
	heartbeatTicker := time.NewTicker(30 * time.Second)
	reporterTicker := time.NewTicker(60 * time.Second)
	defer func() {
		heartbeatTicker.Stop()
		reporterTicker.Stop()
	}()

	lastHeartbeatHealth := time.Now()
	lastReporterHealth := time.Now()

	for {
		select {
		case <-pm.ctx.Done():
			return

		case health := <-pm.heartbeatHealth:
			if health {
				lastHeartbeatHealth = time.Now()
				pm.heartbeatRestartDelay = 1 * time.Second // 重置延迟
			} else {
				pm.logger.Warn("心跳进程：健康检查失败")
			}

		case health := <-pm.reporterHealth:
			if health {
				lastReporterHealth = time.Now()
				pm.reporterRestartDelay = 1 * time.Second // 重置延迟
			} else {
				pm.logger.Warn("数据上报进程：健康检查失败")
			}

		case <-heartbeatTicker.C:
			// 检查心跳进程是否超时（超过60秒没有健康信号）
			if time.Since(lastHeartbeatHealth) > 60*time.Second {
				pm.logger.Warn("心跳进程：健康检查超时，准备重启")
				pm.restartHeartbeat()
			}

		case <-reporterTicker.C:
			// 检查上报进程是否超时（超过120秒没有健康信号）
			if time.Since(lastReporterHealth) > 120*time.Second {
				pm.logger.Warn("数据上报进程：健康检查超时，准备重启")
				pm.restartReporter()
			}
		}
	}
}

// restartHeartbeat 重启心跳进程
func (pm *ProcessManager) restartHeartbeat() {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if pm.heartbeatCancel != nil {
		pm.heartbeatCancel()
	}
	pm.StartHeartbeatProcess()
}

// restartReporter 重启数据上报进程
func (pm *ProcessManager) restartReporter() {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if pm.reporterCancel != nil {
		pm.reporterCancel()
	}
	pm.StartReporterProcess()
}

// exponentialBackoff 指数退避算法
func (pm *ProcessManager) exponentialBackoff(delay *time.Duration) {
	*delay *= 2
	if *delay > pm.maxRestartDelay {
		*delay = pm.maxRestartDelay
	}
}

// Shutdown 优雅关闭所有子进程
func (pm *ProcessManager) Shutdown() {
	pm.logger.Info("进程管理器：开始关闭所有子进程...")

	// 取消主 context
	pm.cancel()

	// 停止所有子进程
	pm.mu.Lock()
	if pm.heartbeatCancel != nil {
		pm.heartbeatCancel()
	}
	if pm.reporterCancel != nil {
		pm.reporterCancel()
	}
	pm.mu.Unlock()

	// 等待所有 goroutine 退出
	done := make(chan struct{})
	go func() {
		pm.wg.Wait()
		close(done)
	}()

	// 设置超时
	select {
	case <-done:
		pm.logger.Info("进程管理器：所有子进程已关闭")
	case <-time.After(10 * time.Second):
		pm.logger.Warn("进程管理器：等待子进程关闭超时")
	}
}

// StopHeartbeat 停止心跳进程
func (pm *ProcessManager) StopHeartbeat() {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if pm.heartbeatCancel != nil {
		pm.heartbeatCancel()
		pm.heartbeatCancel = nil
	}
	pm.logger.Info("心跳进程：已停止")
}

// StopReporter 停止数据上报进程
func (pm *ProcessManager) StopReporter() {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if pm.reporterCancel != nil {
		pm.reporterCancel()
		pm.reporterCancel = nil
	}
	pm.logger.Info("数据上报进程：已停止")
}
