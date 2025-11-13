package daemon

import (
	"fmt"
	"os"
	"syscall"
)

// SendSignal 向指定PID的进程发送信号
func SendSignal(pid int, sig syscall.Signal) error {
	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("查找进程失败: %w", err)
	}

	if err := process.Signal(sig); err != nil {
		return fmt.Errorf("发送信号失败: %w", err)
	}

	return nil
}
