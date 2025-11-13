package collector

import (
	"agent/internal/logger"
	"agent/internal/system"
	"agent/internal/websocket"
	"context"
	"runtime"
	"sync"
	"time"

	"github.com/shirou/gopsutil/disk"
	"github.com/shirou/gopsutil/net"
)

const agentVersion = "0.0.1"

type Collector struct {
	System *system.System
	Logger *logger.Logger
	Client *websocket.Client

	// 上报间隔配置
	MetricsInterval int // 性能指标上报间隔（秒）
	DetailInterval  int // 详细信息上报间隔（秒）
	SystemInterval  int // 系统信息上报间隔（秒）

	// 网络IO统计相关
	lastNetIOCounters map[string]net.IOCountersStat
	lastNetIOTime     time.Time
	netIOMutex        sync.RWMutex

	// 磁盘IO统计相关
	lastDiskIOCounters map[string]disk.IOCountersStat
	lastDiskIOTime     time.Time
	diskIOMutex        sync.RWMutex
}

func NewCollector(sys *system.System, log *logger.Logger, client *websocket.Client, metricsInterval, detailInterval, systemInterval int) *Collector {
	return &Collector{
		System:          sys,
		Logger:          log,
		Client:          client,
		MetricsInterval: metricsInterval,
		DetailInterval:  detailInterval,
		SystemInterval:  systemInterval,
	}
}

// SendSystemInfo 发送系统基础信息
func (c *Collector) SendSystemInfo() error {
	hostInfo := c.System.GetHostInfo()
	bootTimeUnix, err := c.System.GetBootTime()
	if err != nil {
		c.Logger.Warn("获取系统启动时间失败: %v", err)
		bootTimeUnix = 0
	}

	var bootTime time.Time
	if bootTimeUnix > 0 {
		bootTime = time.Unix(int64(bootTimeUnix), 0)
	} else {
		bootTime = time.Now()
	}

	systemData := map[string]interface{}{
		"agent_version": agentVersion,
		"system_name":   hostInfo.Platform,
		"os":            hostInfo.OS,
		"architecture":  runtime.GOARCH,
		"kernel":        hostInfo.KernelVersion,
		"hostname":      hostInfo.Hostname,
		"cores":         c.System.GetCpuLogicCount(),
		"boot_time":     bootTime.Format(time.RFC3339),
		"uptime":        hostInfo.Uptime,
	}

	message := websocket.Message{
		Type: "system_info",
		Data: systemData,
	}

	return c.Client.SendMessage(message)
}

// getNetworkSpeed 计算网络速度（字节/秒）
func (c *Collector) getNetworkSpeed() (uploadSpeed float64, downloadSpeed float64) {
	c.netIOMutex.Lock()
	defer c.netIOMutex.Unlock()

	// 获取当前网络IO统计
	currentCounters, err := c.System.GetNetIOCounters()
	if err != nil {
		c.Logger.Warn("获取网络IO统计失败: %v", err)
		return 0.0, 0.0
	}

	// 计算所有网络接口的总发送和接收字节数
	var currentBytesSent uint64 = 0
	var currentBytesRecv uint64 = 0
	for _, counter := range currentCounters {
		currentBytesSent += counter.BytesSent
		currentBytesRecv += counter.BytesRecv
	}

	// 如果是第一次获取，保存当前值并返回0
	if c.lastNetIOCounters == nil || c.lastNetIOTime.IsZero() {
		c.lastNetIOCounters = currentCounters
		c.lastNetIOTime = time.Now()
		return 0.0, 0.0
	}

	// 计算上一次的总字节数
	var lastBytesSent uint64 = 0
	var lastBytesRecv uint64 = 0
	for _, counter := range c.lastNetIOCounters {
		lastBytesSent += counter.BytesSent
		lastBytesRecv += counter.BytesRecv
	}

	// 计算时间差（秒）
	timeDiff := time.Since(c.lastNetIOTime).Seconds()
	if timeDiff <= 0 {
		timeDiff = 1.0 // 避免除零
	}

	// 计算速度（字节/秒）
	uploadSpeed = float64(currentBytesSent-lastBytesSent) / timeDiff
	downloadSpeed = float64(currentBytesRecv-lastBytesRecv) / timeDiff

	// 更新上一次的值
	c.lastNetIOCounters = currentCounters
	c.lastNetIOTime = time.Now()

	return uploadSpeed, downloadSpeed
}

// getDiskIOSpeed 计算磁盘IO速度（字节/秒）
func (c *Collector) getDiskIOSpeed() (readSpeed float64, writeSpeed float64) {
	c.diskIOMutex.Lock()
	defer c.diskIOMutex.Unlock()

	// 获取所有磁盘的IO统计
	currentCounters, err := disk.IOCounters()
	if err != nil {
		c.Logger.Warn("获取磁盘IO统计失败: %v", err)
		return 0.0, 0.0
	}

	// 计算所有磁盘的总读取和写入字节数
	var currentBytesRead uint64 = 0
	var currentBytesWrite uint64 = 0
	for _, counter := range currentCounters {
		currentBytesRead += counter.ReadBytes
		currentBytesWrite += counter.WriteBytes
	}

	// 如果是第一次获取，保存当前值并返回0
	if c.lastDiskIOCounters == nil || c.lastDiskIOTime.IsZero() {
		c.lastDiskIOCounters = currentCounters
		c.lastDiskIOTime = time.Now()
		return 0.0, 0.0
	}

	// 计算上一次的总字节数
	var lastBytesRead uint64 = 0
	var lastBytesWrite uint64 = 0
	for _, counter := range c.lastDiskIOCounters {
		lastBytesRead += counter.ReadBytes
		lastBytesWrite += counter.WriteBytes
	}

	// 计算时间差（秒）
	timeDiff := time.Since(c.lastDiskIOTime).Seconds()
	if timeDiff <= 0 {
		timeDiff = 1.0 // 避免除零
	}

	// 计算速度（字节/秒）
	readSpeed = float64(currentBytesRead-lastBytesRead) / timeDiff
	writeSpeed = float64(currentBytesWrite-lastBytesWrite) / timeDiff

	// 更新上一次的值
	c.lastDiskIOCounters = currentCounters
	c.lastDiskIOTime = time.Now()

	return readSpeed, writeSpeed
}

// getDiskUsage 计算磁盘使用率（返回所有磁盘的平均使用率）
func (c *Collector) getDiskUsage() float64 {
	partitions := c.System.GetDiskPart()
	if len(partitions) == 0 {
		return 0.0
	}

	totalUsage := 0.0
	validCount := 0
	seenDevices := make(map[string]bool) // 用于去重相同设备

	for _, partition := range partitions {
		// 跳过虚拟文件系统
		if c.isVirtualFilesystem(partition.Mountpoint) {
			continue
		}

		// 跳过已处理的设备
		if seenDevices[partition.Device] {
			continue
		}

		usage := c.System.GetDiskUsage(partition.Mountpoint)
		if usage == nil {
			continue
		}

		// 跳过总大小为0的虚拟文件系统
		if usage.Total == 0 {
			continue
		}

		seenDevices[partition.Device] = true
		totalUsage += usage.UsedPercent
		validCount++
	}

	if validCount == 0 {
		return 0.0
	}

	return totalUsage / float64(validCount)
}

// SendMetrics 发送性能指标
func (c *Collector) SendMetrics() error {
	memTotal := c.System.GetMemoryTotal()
	memUsed := c.System.GetMemoryUsed()
	memPercent := c.System.GetMemoryUsedPercent()
	cpuPercent := c.System.GetCpuUsedPercent()

	// 获取网络速度
	networkUpload, networkDownload := c.getNetworkSpeed()

	// 获取磁盘使用率
	diskUsage := c.getDiskUsage()

	metricsData := map[string]interface{}{
		"cpu_usage":            cpuPercent,
		"memory_total":         memTotal,
		"memory_used":          memUsed,
		"memory_usage_percent": memPercent,
		"disk_usage":           diskUsage,
		"network_upload":       networkUpload,
		"network_download":     networkDownload,
	}

	message := websocket.Message{
		Type: "metrics",
		Data: metricsData,
	}

	return c.Client.SendMessage(message)
}

// SendCPUInfo 发送详细CPU信息
func (c *Collector) SendCPUInfo() error {
	cpuPercents := c.System.GetCpuUsedPercentEach()

	cpuInfoList := c.System.GetCpuInfo()
	cpuName := "Unknown CPU"
	if len(cpuInfoList) > 0 {
		cpuName = cpuInfoList[0].ModelName
	}

	if len(cpuPercents) == 0 {
		c.Logger.Warn("未获取到CPU核心使用率数据")
		return nil
	}

	var cpuData []map[string]interface{}
	for coreIndex, usage := range cpuPercents {
		cpuData = append(cpuData, map[string]interface{}{
			"cpu_name":   cpuName,
			"core_index": coreIndex, // 索引
			"cpu_usage":  usage,     // 使用率
		})
	}

	message := websocket.Message{
		Type: "cpu_info",
		Data: cpuData,
	}

	return c.Client.SendMessage(message)
}

// SendMemoryInfo 发送内存历史信息
func (c *Collector) SendMemoryInfo() error {
	memTotal := c.System.GetMemoryTotal()
	memUsed := c.System.GetMemoryUsed()
	memPercent := c.System.GetMemoryUsedPercent()

	memoryData := map[string]interface{}{
		"memory_total":         memTotal,
		"memory_used":          memUsed,
		"memory_usage_percent": memPercent,
	}

	message := websocket.Message{
		Type: "memory_info",
		Data: memoryData,
	}

	return c.Client.SendMessage(message)
}

// isVirtualFilesystem 判断是否为虚拟文件系统
func (c *Collector) isVirtualFilesystem(mountPoint string) bool {
	// 常见的虚拟文件系统挂载点前缀
	virtualMountPrefixes := []string{
		"/proc",
		"/sys",
		"/dev",
		"/run",
		"/var/run",
		"/snap",
	}

	// 检查是否以虚拟文件系统路径开头
	for _, prefix := range virtualMountPrefixes {
		if mountPoint == prefix || (len(mountPoint) > len(prefix) && mountPoint[:len(prefix)+1] == prefix+"/") {
			return true
		}
	}

	return false
}

// SendDiskInfo 发送磁盘信息
func (c *Collector) SendDiskInfo() error {
	partitions := c.System.GetDiskPart()

	var diskData []map[string]interface{}
	seenDevices := make(map[string]bool) // 用于去重相同设备

	for _, partition := range partitions {
		// 跳过虚拟文件系统
		if c.isVirtualFilesystem(partition.Mountpoint) {
			continue
		}

		// 跳过已处理的设备
		if seenDevices[partition.Device] {
			continue
		}

		usage := c.System.GetDiskUsage(partition.Mountpoint)
		if usage == nil {
			continue
		}

		// 跳过总大小为0的虚拟文件系统
		if usage.Total == 0 {
			continue
		}

		seenDevices[partition.Device] = true
		diskData = append(diskData, map[string]interface{}{
			"mount_point":   partition.Mountpoint,
			"device":        partition.Device,
			"total":         usage.Total,
			"used":          usage.Used,
			"free":          usage.Free,
			"usage_percent": usage.UsedPercent,
		})
	}

	message := websocket.Message{
		Type: "disk_info",
		Data: diskData,
	}

	return c.Client.SendMessage(message)
}

// SendDiskIO 发送磁盘IO信息
func (c *Collector) SendDiskIO() error {
	// 获取磁盘IO速度
	readSpeed, writeSpeed := c.getDiskIOSpeed()

	diskIOData := map[string]interface{}{
		"read_speed":  readSpeed,  // 字节/秒
		"write_speed": writeSpeed, // 字节/秒
	}

	message := websocket.Message{
		Type: "disk_io",
		Data: diskIOData,
	}

	return c.Client.SendMessage(message)
}

// SendNetworkInfo 发送网络信息
func (c *Collector) SendNetworkInfo() error {
	connections := c.System.GetNetIO()

	tcpConns := 0
	udpConns := 0

	for _, conn := range connections {
		if conn.Type == 1 { // TCP
			tcpConns++
		} else if conn.Type == 2 { // UDP
			udpConns++
		}
	}

	// 获取网络IO统计
	counters, err := c.System.GetNetIOCounters()
	if err != nil {
		c.Logger.Warn("获取网络IO统计失败: %v", err)
		counters = make(map[string]net.IOCountersStat)
	}

	// 计算所有网络接口的总发送和接收字节数
	var totalBytesSent uint64 = 0
	var totalBytesRecv uint64 = 0
	for _, counter := range counters {
		totalBytesSent += counter.BytesSent
		totalBytesRecv += counter.BytesRecv
	}

	// 获取网络速度
	uploadSpeed, downloadSpeed := c.getNetworkSpeed()

	networkData := map[string]interface{}{
		"tcp_connections": tcpConns,
		"udp_connections": udpConns,
		"upload_speed":    uploadSpeed,
		"download_speed":  downloadSpeed,
		"upload_bytes":    totalBytesSent,
		"download_bytes":  totalBytesRecv,
	}

	message := websocket.Message{
		Type: "network_info",
		Data: networkData,
	}

	return c.Client.SendMessage(message)
}

// SendVirtualMemory 发送虚拟内存信息
func (c *Collector) SendVirtualMemory() error {
	// 这里使用swap信息作为虚拟内存
	// gopsutil库中虚拟内存实际上就是swap
	// 这里可以进一步完善

	vmData := map[string]interface{}{
		"virtual_memory_total": 0,
		"virtual_memory_used":  0,
		"virtual_memory_free":  0,
	}

	message := websocket.Message{
		Type: "virtual_memory",
		Data: vmData,
	}

	return c.Client.SendMessage(message)
}

// StartPeriodicReporting 启动周期性上报，使用 context 控制生命周期
func (c *Collector) StartPeriodicReporting(ctx context.Context, healthChan chan<- bool) {
	// 立即发送一次系统信息
	if err := c.SendSystemInfo(); err != nil {
		c.Logger.Warn("发送系统信息失败: %v", err)
		select {
		case healthChan <- false:
		default:
		}
	} else {
		select {
		case healthChan <- true:
		default:
		}
	}

	// 创建所有 ticker
	metricsTicker := time.NewTicker(time.Duration(c.MetricsInterval) * time.Second)
	detailTicker := time.NewTicker(time.Duration(c.DetailInterval) * time.Second)
	systemTicker := time.NewTicker(time.Duration(c.SystemInterval) * time.Second)

	c.Logger.Info("数据上报间隔配置: 性能指标=%d秒, 详细信息=%d秒, 系统信息=%d秒",
		c.MetricsInterval, c.DetailInterval, c.SystemInterval)

	// 确保所有 ticker 在退出时停止
	defer func() {
		metricsTicker.Stop()
		detailTicker.Stop()
		systemTicker.Stop()
		c.Logger.Info("数据上报进程：已停止")
	}()

	c.Logger.Success("数据上报进程：已启动")

	for {
		select {
		case <-metricsTicker.C:
			// 每30秒发送一次性能指标
			if !c.Client.IsConnected {
				c.Logger.Warn("未连接，跳过性能指标上报")
				select {
				case healthChan <- false:
				default:
				}
				continue
			}
			if err := c.SendMetrics(); err != nil {
				c.Logger.Error("发送性能指标失败: %v", err)
				select {
				case healthChan <- false:
				default:
				}
			} else {
				select {
				case healthChan <- true:
				default:
				}
			}

		case <-detailTicker.C:
			// 每60秒发送一次详细信息
			if !c.Client.IsConnected {
				c.Logger.Warn("未连接，跳过详细信息上报")
				select {
				case healthChan <- false:
				default:
				}
				continue
			}
			hasError := false
			if err := c.SendMemoryInfo(); err != nil {
				c.Logger.Error("发送内存信息失败: %v", err)
				hasError = true
			}
			if err := c.SendDiskInfo(); err != nil {
				c.Logger.Error("发送磁盘信息失败: %v", err)
				hasError = true
			}
			if err := c.SendDiskIO(); err != nil {
				c.Logger.Error("发送磁盘IO失败: %v", err)
				hasError = true
			}
			if err := c.SendNetworkInfo(); err != nil {
				c.Logger.Error("发送网络信息失败: %v", err)
				hasError = true
			}
			if err := c.SendVirtualMemory(); err != nil {
				c.Logger.Error("发送虚拟内存信息失败: %v", err)
				hasError = true
			}
			select {
			case healthChan <- !hasError:
			default:
			}

		case <-systemTicker.C:
			// 每5分钟重新发送一次系统信息
			if !c.Client.IsConnected {
				c.Logger.Warn("未连接，跳过系统信息上报")
				select {
				case healthChan <- false:
				default:
				}
				continue
			}
			if err := c.SendSystemInfo(); err != nil {
				c.Logger.Error("发送系统信息失败: %v", err)
				select {
				case healthChan <- false:
				default:
				}
			} else {
				select {
				case healthChan <- true:
				default:
				}
			}

		case <-ctx.Done():
			return
		}
	}
}

// UpdateIntervals 更新上报间隔配置（用于配置重载）
func (c *Collector) UpdateIntervals(metricsInterval, detailInterval, systemInterval int) {
	c.MetricsInterval = metricsInterval
	c.DetailInterval = detailInterval
	c.SystemInterval = systemInterval
	c.Logger.Info("配置已更新: 性能指标=%d秒, 详细信息=%d秒, 系统信息=%d秒",
		c.MetricsInterval, c.DetailInterval, c.SystemInterval)
}
