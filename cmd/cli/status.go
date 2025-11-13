package cli

import (
	"fmt"
	"os"

	"agent/internal/daemon"
	"agent/internal/systemd"

	"github.com/spf13/cobra"
)

// statusCmd 状态命令
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "查看agent状态",
	Long:  `查看CloudSentinel Agent的运行状态。`,
	RunE:  runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) error {
	// 检查systemd服务状态
	if systemd.ServiceExists() {
		active, err := systemd.IsServiceActive()
		if err == nil {
			if active {
				fmt.Println("状态: 运行中 (systemd管理)")
				return nil
			} else {
				fmt.Println("状态: 已停止 (systemd管理)")
				return nil
			}
		}
	}

	// 检查PID文件
	pid, running, err := daemon.CheckPIDFile(pidFile)
	if err != nil {
		return fmt.Errorf("检查PID文件失败: %w", err)
	}

	if running {
		fmt.Printf("状态: 运行中 (PID: %d)\n", pid)
		os.Exit(0)
	} else {
		fmt.Println("状态: 已停止")
		os.Exit(1)
	}

	return nil
}
