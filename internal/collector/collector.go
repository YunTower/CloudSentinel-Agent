package collector

import (
	"agent/internal/logger"
	"agent/internal/system"
	"agent/internal/websocket"
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

	// 网络IO统计相关
	lastNetIOCounters map[string]net.IOCountersStat
	lastNetIOTime     time.Time
	netIOMutex        sync.RWMutex

	// 磁盘IO统计相关
	lastDiskIOCounters map[string]disk.IOCountersStat
	lastDiskIOTime     time.Time
	diskIOMutex        sync.RWMutex
}

func NewCollector(sys *system.System, log *logger.Logger, client *websocket.Client) *Collector {
	return &Collector{
		System: sys,
		Logger: log,
		Client: client,
	}
}

// SendSystemInfo 发送系统基础信息
func (c *Collector) SendSystemInfo() error {
	hostInfo := c.System.GetHostInfo()

	bootTime := time.Unix(int64(hostInfo.BootTime), 0)

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

	for _, partition := range partitions {
		usage := c.System.GetDiskUsage(partition.Mountpoint)
		if usage == nil {
			continue
		}
		totalUsage += usage.UsedPercent
		validCount++
	}

	if validCount == 0 {
		return 0.0
	}

	return totalUsage / float64(validCount)
}

// SendMetrics 发送性能指标（用于实时监控）
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

// SendDiskInfo 发送磁盘信息
func (c *Collector) SendDiskInfo() error {
	partitions := c.System.GetDiskPart()

	var diskData []map[string]interface{}
	for _, partition := range partitions {
		usage := c.System.GetDiskUsage(partition.Mountpoint)
		if usage == nil {
			continue
		}

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

// StartPeriodicReporting 启动周期性上报
func (c *Collector) StartPeriodicReporting() {
	// 立即发送一次系统信息
	c.SendSystemInfo()

	// 每30秒发送一次性能指标
	metricsTicker := time.NewTicker(30 * time.Second)
	go func() {
		for range metricsTicker.C {
			if !c.Client.IsConnected {
				c.Logger.Warn("未连接，跳过性能指标上报")
				continue
			}
			c.SendMetrics()
		}
	}()

	// 每60秒发送一次详细信息
	detailTicker := time.NewTicker(60 * time.Second)
	go func() {
		for range detailTicker.C {
			if !c.Client.IsConnected {
				c.Logger.Warn("未连接，跳过详细信息上报")
				continue
			}
			c.SendMemoryInfo()
			c.SendDiskInfo()
			c.SendDiskIO() // 发送磁盘IO数据
			c.SendNetworkInfo()
			c.SendVirtualMemory()
		}
	}()

	// 每5分钟重新发送一次系统信息
	systemTicker := time.NewTicker(5 * time.Minute)
	go func() {
		for range systemTicker.C {
			if !c.Client.IsConnected {
				c.Logger.Warn("未连接，跳过系统信息上报")
				continue
			}
			c.SendSystemInfo()
		}
	}()

	c.Logger.Success("周期性上报已启动")
}
