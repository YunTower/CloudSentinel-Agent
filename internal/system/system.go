package system

import (
	"agent/internal/logger"
	"io"
	"net/http"
	"time"

	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/disk"
	"github.com/shirou/gopsutil/host"
	"github.com/shirou/gopsutil/mem"
	"github.com/shirou/gopsutil/net"
)

type System struct {
}

// GetHostInfo 本机信息
func (s *System) GetHostInfo() *host.InfoStat {
	h, _ := host.Info()
	return h
}

// GetBootTime 获取系统启动时间
func (s *System) GetBootTime() (uint64, error) {
	return host.BootTime()
}

// GetMemoryTotal 总内存
func (s *System) GetMemoryTotal() int {
	v, _ := mem.VirtualMemory()
	return int(v.Total)
}

// GetMemoryFree 剩余内存
func (s *System) GetMemoryFree() int {
	v, _ := mem.VirtualMemory()
	return int(v.Free)
}

// GetMemoryUsed 已使用的内存
func (s *System) GetMemoryUsed() int {
	v, _ := mem.VirtualMemory()
	return int(v.Used)
}

// GetMemoryUsedPercent 内存占用百分比
func (s *System) GetMemoryUsedPercent() int {
	v, _ := mem.VirtualMemory()
	return int(v.UsedPercent)
}

// GetCpuCount cpu 物理核心数
func (s *System) GetCpuCount() int {
	count, _ := cpu.Counts(false)
	return int(count)
}

// GetCpuLogicCount cpu 逻辑核心数
func (s *System) GetCpuLogicCount() int {
	count, _ := cpu.Counts(true)
	return int(count)
}

// GetCpuUsedPercent 3s内的cpu总使用率
func (s *System) GetCpuUsedPercent() int {
	percent, _ := cpu.Percent(3*time.Second, false)
	return int(percent[0])
}

// GetCpuUsedPercentEach 3s内每个cpu核心的使用率
func (s *System) GetCpuUsedPercentEach() []float64 {
	percent, _ := cpu.Percent(3*time.Second, true)
	return percent
}

// GetCpuInfo cpu信息
func (s *System) GetCpuInfo() []cpu.InfoStat {
	info, _ := cpu.Info()
	return info
}

// GetDiskPart 获取磁盘分区
func (s *System) GetDiskPart() []disk.PartitionStat {
	part, _ := disk.Partitions(false)
	return part
}

// GetDiskAllPart 获取磁盘所有分区
func (s *System) GetDiskAllPart() []disk.PartitionStat {
	part, _ := disk.Partitions(true)
	return part
}

// GetDiskIO 获取所有硬盘的IO信息
func (s *System) GetDiskIO(path string) map[string]disk.IOCountersStat {
	io, _ := disk.IOCounters(path)
	return io
}

// GetDiskUsage 获取磁盘使用信息
func (s *System) GetDiskUsage(path string) *disk.UsageStat {
	usage, _ := disk.Usage(path)
	return usage
}

// GetNetIO 获取当前网络连接信息
func (s *System) GetNetIO() []net.ConnectionStat {
	netInfo, _ := net.Connections("all")
	return netInfo
}

// GetNetIOCounters 获取网络IO计数器（用于计算网络速度）
func (s *System) GetNetIOCounters() (map[string]net.IOCountersStat, error) {
	counters, err := net.IOCounters(true) // true表示获取所有网络接口
	if err != nil {
		return nil, err
	}

	result := make(map[string]net.IOCountersStat)
	for _, counter := range counters {
		result[counter.Name] = counter
	}
	return result, nil
}

// GetIpv4 获取本机ipv4地址
func (s *System) GetIpv4(log *logger.Logger) string {
	urls := []string{
		"https://api.ipify.org",
		"https://4.ipw.cn",
	}

	log.Info("获取本机IPv4...")
	for _, url := range urls {
		resp, err := http.Get(url)
		if err != nil {
			log.Warn("获取IPv4失败，切换接口（url: %s error: %v）", url, err)
			continue
		}
		defer resp.Body.Close()

		ip, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Error("获取 %s 响应失败: %v\n", url, err)
			continue
		}

		return string(ip)
	}

	log.Error("无法获取服务器IPv4地址")
	return ""
}
