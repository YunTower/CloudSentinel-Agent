package system

import (
	"strings"
	"time"

	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/disk"
	"github.com/shirou/gopsutil/host"
	"github.com/shirou/gopsutil/mem"
	"github.com/shirou/gopsutil/net"
	"github.com/shirou/gopsutil/process"
)

type System struct {
}

// ProcessStatus 进程状态
type ProcessStatus struct {
	Name    string  `json:"name"`
	Running bool    `json:"running"`
	Pids    []int32 `json:"pids"`
	CPU     float64 `json:"cpu"`
	Memory  float64 `json:"memory"`
}

// GetHostInfo 本机信息
func (s *System) GetHostInfo() *host.InfoStat {
	h, _ := host.Info()
	return h
}

// GetBootTime 获取系统启动时间（Unix时间戳）
func (s *System) GetBootTime() (uint64, error) {
	return host.BootTime()
}

// GetUptime 获取系统运行时间
func (s *System) GetUptime() uint64 {
	hostInfo := s.GetHostInfo()
	if hostInfo != nil && hostInfo.Uptime > 0 {
		return hostInfo.Uptime
	}

	// 如果 hostInfo.Uptime 不可用，从 boot_time 计算
	bootTimeUnix, err := s.GetBootTime()
	if err != nil || bootTimeUnix == 0 {
		return 0
	}

	// 计算从 boot_time 到现在的秒数
	now := time.Now().Unix()
	if int64(bootTimeUnix) > now {
		return 0 // boot_time 异常，返回0
	}

	return uint64(now - int64(bootTimeUnix))
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

// GetSwapMemory 获取Swap内存信息
func (s *System) GetSwapMemory() (total, used, free int, usedPercent float64) {
	swap, err := mem.SwapMemory()
	if err != nil {
		return 0, 0, 0, 0.0
	}
	return int(swap.Total), int(swap.Used), int(swap.Free), swap.UsedPercent
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
	if len(percent) > 0 {
		return int(percent[0])
	}
	return 0
}

// GetDiskInfo 磁盘信息
func (s *System) GetDiskInfo() []disk.UsageStat {
	parts, _ := disk.Partitions(true)
	var disks []disk.UsageStat
	for _, part := range parts {
		u, _ := disk.Usage(part.Mountpoint)
		disks = append(disks, *u)
	}
	return disks
}

// GetDiskIOCounters 磁盘IO信息
func (s *System) GetDiskIOCounters() (map[string]disk.IOCountersStat, error) {
	return disk.IOCounters()
}

// GetDiskPart 获取磁盘分区信息
func (s *System) GetDiskPart() []disk.PartitionStat {
	parts, _ := disk.Partitions(true)
	return parts
}

// GetDiskUsage 获取指定挂载点的磁盘使用情况
func (s *System) GetDiskUsage(mountpoint string) *disk.UsageStat {
	usage, err := disk.Usage(mountpoint)
	if err != nil {
		return nil
	}
	return usage
}

// GetNetIOCounters 网络IO信息
func (s *System) GetNetIOCounters() (map[string]net.IOCountersStat, error) {
	return net.IOCounters(true)
}

// GetProcessStatus 获取指定服务的状态
func (s *System) GetProcessStatus(services []string) ([]ProcessStatus, error) {
	if len(services) == 0 {
		return []ProcessStatus{}, nil
	}

	processes, err := process.Processes()
	if err != nil {
		return nil, err
	}

	statusMap := make(map[string]*ProcessStatus)
	for _, name := range services {
		statusMap[name] = &ProcessStatus{Name: name, Running: false, Pids: []int32{}}
	}

	for _, p := range processes {
		name, err := p.Name()
		if err != nil {
			continue
		}

		// 简单的名称匹配，可能需要更复杂的逻辑（如匹配命令行参数）
		for _, serviceName := range services {
			if strings.Contains(strings.ToLower(name), strings.ToLower(serviceName)) {
				s := statusMap[serviceName]
				s.Running = true
				s.Pids = append(s.Pids, p.Pid)

				cpuPercent, _ := p.CPUPercent()
				s.CPU += cpuPercent

				memPercent, _ := p.MemoryPercent()
				s.Memory += float64(memPercent)
			}
		}
	}

	var result []ProcessStatus
	for _, name := range services {
		result = append(result, *statusMap[name])
	}
	return result, nil
}
