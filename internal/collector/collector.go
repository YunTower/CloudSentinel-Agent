package collector

import (
	"agent/internal/logger"
	"agent/internal/system"
	"agent/internal/websocket"
	"runtime"
	"time"
)

const agentVersion = "0.0.1"

type Collector struct {
	System *system.System
	Logger *logger.Logger
	Client *websocket.Client
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
	
	c.Logger.Info("发送系统信息")
	return c.Client.SendMessage(message)
}

// SendMetrics 发送性能指标（用于实时监控）
func (c *Collector) SendMetrics() error {
	memTotal := c.System.GetMemoryTotal()
	memUsed := c.System.GetMemoryUsed()
	memPercent := c.System.GetMemoryUsedPercent()
	cpuPercent := c.System.GetCpuUsedPercent()
	
	// 获取网络统计（这里需要实现网络速度计算）
	// 暂时使用占位符
	networkUpload := 0.0
	networkDownload := 0.0
	
	metricsData := map[string]interface{}{
		"cpu_usage":            cpuPercent,
		"memory_total":         memTotal,
		"memory_used":          memUsed,
		"memory_usage_percent": memPercent,
		"network_upload":       networkUpload,
		"network_download":     networkDownload,
	}
	
	message := websocket.Message{
		Type: "metrics",
		Data: metricsData,
	}
	
	c.Logger.Info("发送性能指标: CPU=%d%%, MEM=%d%%", cpuPercent, memPercent)
	return c.Client.SendMessage(message)
}

// SendCPUInfo 发送详细CPU信息
func (c *Collector) SendCPUInfo() error {
	cpuInfoList := c.System.GetCpuInfo()
	cpuPercents := c.System.GetCpuUsedPercentEach()
	
	var cpuData []map[string]interface{}
	for i, info := range cpuInfoList {
		usage := 0.0
		if i < len(cpuPercents) {
			usage = cpuPercents[i]
		}
		
		cpuData = append(cpuData, map[string]interface{}{
			"cpu_name":  info.ModelName,
			"cpu_usage": usage,
			"cores":     info.Cores,
		})
	}
	
	message := websocket.Message{
		Type: "cpu_info",
		Data: cpuData,
	}
	
	// c.Logger.Info("发送CPU详细信息")
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
	
	// c.Logger.Info("发送内存信息")
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
	
	// c.Logger.Info("发送磁盘信息")
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
	
	// 这里需要添加流量和速度统计
	// 暂时使用占位符
	networkData := map[string]interface{}{
		"tcp_connections": tcpConns,
		"udp_connections": udpConns,
		"upload_speed":    0.0,
		"download_speed":  0.0,
		"upload_bytes":    0,
		"download_bytes":  0,
	}
	
	message := websocket.Message{
		Type: "network_info",
		Data: networkData,
	}
	
	// c.Logger.Info("发送网络信息: TCP=%d, UDP=%d", tcpConns, udpConns)
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
	
	// c.Logger.Info("发送虚拟内存信息")
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
			c.SendCPUInfo()
			c.SendMemoryInfo()
			c.SendDiskInfo()
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

