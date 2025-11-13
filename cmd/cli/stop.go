package cli

import (
	"fmt"
	"syscall"
	"time"

	"agent/internal/daemon"

	"github.com/spf13/cobra"
)

// stopCmd 停止命令
var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "停止agent",
	Long:  `停止正在运行的CloudSentinel Agent。`,
	RunE:  runStop,
}

func init() {
	rootCmd.AddCommand(stopCmd)
}

func runStop(cmd *cobra.Command, args []string) error {
	// 读取PID
	pid, running, err := daemon.CheckPIDFile(pidFile)
	if err != nil {
		return fmt.Errorf("检查PID文件失败: %w", err)
	}

	if !running {
		fmt.Println("agent未运行")
		// 清理可能残留的PID文件
		daemon.RemovePID(pidFile)
		return nil
	}

	// 发送SIGTERM信号
	if err := daemon.SendSignal(pid, syscall.SIGTERM); err != nil {
		return fmt.Errorf("发送停止信号失败: %w", err)
	}

	fmt.Printf("已向agent发送停止信号 (PID: %d)\n", pid)

	// 等待进程退出（最多等待10秒）
	for i := 0; i < 10; i++ {
		if !daemon.IsProcessRunning(pid) {
			fmt.Println("agent已停止")
			daemon.RemovePID(pidFile)
			return nil
		}
		// 等待1秒
		time.Sleep(1 * time.Second)
	}

	// 如果还在运行，尝试强制终止
	if daemon.IsProcessRunning(pid) {
		if err := daemon.SendSignal(pid, syscall.SIGKILL); err != nil {
			return fmt.Errorf("强制终止失败: %w", err)
		}
		fmt.Println("agent已强制终止")
		daemon.RemovePID(pidFile)
	}

	return nil
}
