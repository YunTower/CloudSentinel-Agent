package cli

import (
	"fmt"
	"os"

	"agent/internal/systemd"

	"github.com/spf13/cobra"
)

// uninstallCmd 卸载命令
var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "卸载systemd服务",
	Long:  `卸载CloudSentinel Agent的systemd服务（需要root权限）。`,
	RunE:  runUninstall,
}

func init() {
	rootCmd.AddCommand(uninstallCmd)
}

func runUninstall(cmd *cobra.Command, args []string) error {
	// 检查是否为root用户
	if os.Geteuid() != 0 {
		return fmt.Errorf("卸载systemd服务需要root权限，请使用sudo运行")
	}

	// 检查service是否存在
	if !systemd.ServiceExists() {
		printInfo("systemd服务未安装")
		return nil
	}

	// 停止服务
	if active, _ := systemd.IsServiceActive(); active {
		if err := systemd.StopService(); err != nil {
			printWarning(fmt.Sprintf("停止服务时出现警告: %v", err))
		}
	}

	// 禁用服务
	if err := systemd.DisableService(); err != nil {
		printWarning(fmt.Sprintf("禁用服务时出现警告: %v", err))
	}

	// 删除service文件
	if err := systemd.UninstallService(); err != nil {
		printError(fmt.Sprintf("删除service文件失败: %v", err))
		return err
	}

	// 重新加载systemd daemon
	if err := systemd.ReloadDaemon(); err != nil {
		printError(fmt.Sprintf("重新加载systemd daemon失败: %v", err))
		return err
	}

	printSuccess("systemd服务卸载成功")
	return nil
}
