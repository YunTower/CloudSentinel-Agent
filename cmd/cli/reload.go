package cli

import (
	"fmt"
	"syscall"

	"agent/internal/daemon"

	"github.com/spf13/cobra"
)

// reloadCmd 重载命令
var reloadCmd = &cobra.Command{
	Use:   "reload",
	Short: "重载配置",
	Long:  `向正在运行的agent发送SIGHUP信号以重载配置。`,
	RunE:  runReload,
}

func init() {
	rootCmd.AddCommand(reloadCmd)
}

func runReload(cmd *cobra.Command, args []string) error {
	// 读取PID
	pid, running, err := daemon.CheckPIDFile(pidFile)
	if err != nil {
		return fmt.Errorf("检查PID文件失败: %w", err)
	}

	if !running {
		return fmt.Errorf("agent未运行")
	}

	// 发送SIGHUP信号
	if err := daemon.SendSignal(pid, syscall.SIGHUP); err != nil {
		return fmt.Errorf("发送重载信号失败: %w", err)
	}

	fmt.Printf("已向agent发送重载信号 (PID: %d)\n", pid)
	return nil
}
