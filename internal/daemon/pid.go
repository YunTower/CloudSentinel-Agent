package daemon

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
)

const (
	// PIDFile 默认PID文件路径
	PIDFile = "/var/run/cloudsentinel-agent.pid"
)

// WritePID 写入PID到文件
func WritePID(pidFile string) error {
	pid := os.Getpid()
	pidStr := fmt.Sprintf("%d\n", pid)

	// 确保目录存在
	dir := filepath.Dir(pidFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建PID目录失败: %w", err)
	}

	// 写入PID文件
	if err := os.WriteFile(pidFile, []byte(pidStr), 0644); err != nil {
		return fmt.Errorf("写入PID文件失败: %w", err)
	}

	return nil
}

// ReadPID 从文件读取PID
func ReadPID(pidFile string) (int, error) {
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return 0, fmt.Errorf("读取PID文件失败: %w", err)
	}

	pid, err := strconv.Atoi(string(data[:len(data)-1])) // 去掉换行符
	if err != nil {
		return 0, fmt.Errorf("解析PID失败: %w", err)
	}

	return pid, nil
}

// RemovePID 删除PID文件
func RemovePID(pidFile string) error {
	if err := os.Remove(pidFile); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("删除PID文件失败: %w", err)
	}
	return nil
}

// IsProcessRunning 检查进程是否正在运行
func IsProcessRunning(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	// 发送信号0来检查进程是否存在
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// CheckPIDFile 检查PID文件是否存在且进程正在运行
func CheckPIDFile(pidFile string) (int, bool, error) {
	// 先检查文件是否存在
	_, err := os.Stat(pidFile)
	if err != nil {
		if os.IsNotExist(err) {
			// 文件不存在，表示没有运行中的实例
			return 0, false, nil
		}
		// 其他错误（如权限问题）
		return 0, false, fmt.Errorf("检查PID文件失败: %w", err)
	}

	// 文件存在，读取PID
	pid, err := ReadPID(pidFile)
	if err != nil {
		// 读取失败，可能是文件损坏，清理并返回未运行
		os.Remove(pidFile)
		return 0, false, nil
	}

	// 检查进程是否运行
	running := IsProcessRunning(pid)
	return pid, running, nil
}
