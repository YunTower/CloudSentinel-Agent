package cli

import (
	"fmt"

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
	// 优先检查systemd服务状态
	if systemd.ServiceExists() {
		status, err := systemd.GetServiceStatus()
		if err == nil {
			switch status {
			case "active":
				printStatus("running", "状态: 运行中")
				return nil
			case "inactive":
				printStatus("stopped", "状态: 已停止")
				printInfo("使用 'sudo ./agent start' 启动服务")
				printInfo("使用 './agent logs' 查看日志")
				return fmt.Errorf("agent未运行")
			case "activating":
				printStatus("starting", "状态: 正在启动")
				return nil
			case "deactivating":
				printStatus("stopping", "状态: 正在停止")
				return nil
			case "failed":
				printStatus("failed", "状态: 启动失败")
				printWarning("使用 './agent logs' 查看错误日志")
				return fmt.Errorf("agent启动失败")
			default:
				printStatus(status, fmt.Sprintf("状态: %s", status))
				return fmt.Errorf("agent状态异常: %s", status)
			}
		}
		// 如果获取systemd状态失败，继续检查PID文件
	}

	// 检查PID文件
	pid, running, err := daemon.CheckPIDFile(pidFile)
	if err != nil {
		// PID文件检查失败，可能文件不存在或格式错误
		// 如果systemd服务也不存在，说明agent确实未运行
		printStatus("stopped", "状态: 已停止")
		return fmt.Errorf("agent未运行")
	}

	if running {
		printStatus("running", fmt.Sprintf("状态: 运行中 (PID: %d)", pid))
		return nil
	} else {
		printStatus("stopped", "状态: 已停止")
		return fmt.Errorf("agent未运行")
	}
}
