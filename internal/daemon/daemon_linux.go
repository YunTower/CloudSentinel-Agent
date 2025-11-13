//go:build linux
// +build linux

package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

// Daemonize 将当前进程转换为守护进程（仅Linux）
func Daemonize() error {
	// 检查是否已经是守护进程
	if os.Getppid() == 1 {
		return nil // 已经是守护进程
	}

	// 获取可执行文件路径
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("获取可执行文件路径失败: %w", err)
	}

	// 获取绝对路径
	execPath, err = filepath.Abs(execPath)
	if err != nil {
		return fmt.Errorf("获取绝对路径失败: %w", err)
	}

	// 创建新的命令，传递所有参数
	cmd := exec.Command(execPath, os.Args[1:]...)
	cmd.Env = os.Environ()

	// 设置进程属性（仅Linux）
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true, // 创建新的会话（仅Linux支持）
	}

	// 重定向标准输入输出到/dev/null
	nullFile, err := os.OpenFile("/dev/null", os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("打开/dev/null失败: %w", err)
	}
	defer nullFile.Close()

	cmd.Stdin = nullFile
	cmd.Stdout = nullFile
	cmd.Stderr = nullFile

	// 启动守护进程
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动守护进程失败: %w", err)
	}

	// 父进程退出
	os.Exit(0)
	return nil
}
