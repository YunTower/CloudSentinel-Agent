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
		status, err := systemd.GetServiceStatus()
		if err == nil {
			switch status {
			case "active":
				printStatus("running", "状态: 运行中")
				os.Exit(0)
			case "inactive":
				printStatus("stopped", "状态: 已停止")
				printInfo("使用 'sudo ./agent start' 启动服务")
				printInfo("使用 './agent logs' 查看日志")
				os.Exit(1)
			case "activating":
				printStatus("starting", "状态: 正在启动")
				os.Exit(0)
			case "deactivating":
				printStatus("stopping", "状态: 正在停止")
				os.Exit(0)
			case "failed":
				printStatus("failed", "状态: 启动失败")
				printWarning("使用 './agent logs' 查看错误日志")
				os.Exit(1)
			default:
				printStatus(status, fmt.Sprintf("状态: %s", status))
				os.Exit(1)
			}
		}
	}

	// 检查PID文件
	pid, running, err := daemon.CheckPIDFile(pidFile)
	if err != nil {
		return fmt.Errorf("检查PID文件失败: %w", err)
	}

	if running {
		printStatus("running", fmt.Sprintf("状态: 运行中 (PID: %d)", pid))
		os.Exit(0)
	} else {
		printStatus("stopped", "状态: 已停止")
		os.Exit(1)
	}

	return nil
}
