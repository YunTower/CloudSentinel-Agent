package cli

import (
	"fmt"
	"os"
	"syscall"
	"time"

	"agent/internal/daemon"
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

	pid, running, err := daemon.CheckPIDFile(pidFile)
	if err != nil {
		// PID文件检查失败，继续尝试启动
		printWarning(fmt.Sprintf("检查PID文件失败: %v", err))
	} else if running {
		// 发送SIGTERM信号
		if err := daemon.SendSignal(pid, syscall.SIGTERM); err != nil {
			printWarning(fmt.Sprintf("发送停止信号失败: %v", err))
		} else {
			printInfo(fmt.Sprintf("已发送停止信号 (PID: %d)", pid))

			// 快速等待进程退出（最多等待2秒，每次检查200ms）
			maxWait := 2 * time.Second
			checkInterval := 200 * time.Millisecond
			elapsed := time.Duration(0)

			for elapsed < maxWait {
				if !daemon.IsProcessRunning(pid) {
					printSuccess("agent已停止")
					daemon.RemovePID(pidFile)
					break
				}
				time.Sleep(checkInterval)
				elapsed += checkInterval
			}

			// 如果还在运行，尝试强制终止
			if daemon.IsProcessRunning(pid) {
				if err := daemon.SendSignal(pid, syscall.SIGKILL); err != nil {
					printWarning(fmt.Sprintf("强制终止失败: %v", err))
				} else {
					printWarning("agent已强制终止")
					daemon.RemovePID(pidFile)
				}
			}
		}
	}

	// 再启动
	startCmd := &cobra.Command{}
	startCmd.SetArgs([]string{})
	return runStart(startCmd, []string{})
}
