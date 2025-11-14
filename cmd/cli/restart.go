package cli

import (
	"fmt"
	"os"

	"agent/internal/systemd"

	"github.com/spf13/cobra"
)

// restartCmd 重启命令
var restartCmd = &cobra.Command{
	Use:   "restart",
	Short: "重启agent",
	Long:  `重启CloudSentinel Agent（先停止，再启动）。`,
	RunE:  runRestart,
}

func init() {
	restartCmd.Flags().BoolVarP(&daemonFlag, "daemon", "d", false, "以守护进程模式运行")
	rootCmd.AddCommand(restartCmd)
}

func runRestart(cmd *cobra.Command, args []string) error {
	// 如果systemd服务存在，优先使用systemd管理
	if systemd.ServiceExists() {
		// 需要root权限操作systemd服务
		if os.Geteuid() != 0 {
			printWarning("需要root权限重启服务")
			printInfo("请使用: sudo ./agent restart")
			return fmt.Errorf("需要root权限")
		}

		// 使用systemd重启
		if err := systemd.RestartService(); err != nil {
			printError(fmt.Sprintf("重启失败: %v", err))
			return err
		}

		printSuccess("agent已重启")
		return nil
	}

	// 直接启动模式：先停止
	stopCmd := &cobra.Command{}
	stopCmd.SetArgs([]string{})
	if err := runStop(stopCmd, []string{}); err != nil {
		// 如果停止失败，继续尝试启动
		printWarning(fmt.Sprintf("停止agent时出现警告: %v", err))
	}

	// 再启动
	startCmd := &cobra.Command{}
	startCmd.SetArgs([]string{})
	return runStart(startCmd, []string{})
}
