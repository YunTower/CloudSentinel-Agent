package system

import (
	"bytes"
	"context"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// GPUInfo GPU信息
type GPUInfo struct {
	Index       int     `json:"index"`
	Name        string  `json:"name"`
	Temperature float64 `json:"temperature"`
	MemoryUsed  uint64  `json:"memory_used"`
	MemoryTotal uint64  `json:"memory_total"`
	MemoryUtil  float64 `json:"memory_util"`
	GPUUtil     float64 `json:"gpu_util"`
}

// GPUStats GPU统计信息
type GPUStats struct {
	Available bool      `json:"available"`
	GPUs      []GPUInfo `json:"gpus"`
}

// GetGPUInfo 获取GPU信息
func (s *System) GetGPUInfo() (*GPUStats, error) {
	stats := &GPUStats{
		Available: false,
		GPUs:      []GPUInfo{},
	}

	// 检测 nvidia-smi 是否存在
	nvidiaSMIPath, err := exec.LookPath("nvidia-smi")
	if err != nil {
		// nvidia-smi 不存在，返回不可用状态（不是错误）
		return stats, nil
	}

	// 创建带超时的上下文（5秒超时）
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 执行 nvidia-smi 命令获取GPU信息
	// 格式：index,name,temperature.gpu,memory.used,memory.total,utilization.gpu,utilization.memory
	cmd := exec.CommandContext(ctx, nvidiaSMIPath,
		"--query-gpu=index,name,temperature.gpu,memory.used,memory.total,utilization.gpu,utilization.memory",
		"--format=csv,noheader,nounits")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// 命令执行失败（可能是超时或GPU驱动异常），返回不可用状态
		return stats, nil
	}

	// 解析输出
	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// 解析CSV行：index,name,temperature,memory_used,memory_total,gpu_util,memory_util
		fields := strings.Split(line, ",")
		if len(fields) < 7 {
			continue
		}

		// 清理每个字段的空格
		for i := range fields {
			fields[i] = strings.TrimSpace(fields[i])
		}

		// 解析各个字段
		index, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}

		name := fields[1]

		temperature, err := strconv.ParseFloat(fields[2], 64)
		if err != nil {
			temperature = 0
		}

		memoryUsed, err := strconv.ParseUint(fields[3], 10, 64)
		if err != nil {
			memoryUsed = 0
		}

		memoryTotal, err := strconv.ParseUint(fields[4], 10, 64)
		if err != nil {
			memoryTotal = 0
		}

		gpuUtil, err := strconv.ParseFloat(fields[5], 64)
		if err != nil {
			gpuUtil = 0
		}

		memoryUtil, err := strconv.ParseFloat(fields[6], 64)
		if err != nil {
			memoryUtil = 0
		}

		stats.GPUs = append(stats.GPUs, GPUInfo{
			Index:       index,
			Name:        name,
			Temperature: temperature,
			MemoryUsed:  memoryUsed,
			MemoryTotal: memoryTotal,
			MemoryUtil:  memoryUtil,
			GPUUtil:     gpuUtil,
		})
	}

	// 如果成功解析到GPU信息，标记为可用
	if len(stats.GPUs) > 0 {
		stats.Available = true
	}

	return stats, nil
}
