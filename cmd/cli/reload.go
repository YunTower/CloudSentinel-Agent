package cli

import (
	"fmt"
	"os"
	"syscall"

	"agent/internal/daemon"
	"agent/internal/systemd"

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
	// 优先检查systemd服务
	if systemd.ServiceExists() {
		active, err := systemd.IsServiceActive()
		if err != nil {
			// 检查systemd状态失败，继续尝试PID文件方式
			printWarning(fmt.Sprintf("检查systemd服务状态失败: %v，尝试使用PID文件方式", err))
		} else {
			if !active {
				printStatus("stopped", "agent未运行")
				return fmt.Errorf("agent未运行")
			}

			// 需要root权限操作systemd服务
			if os.Geteuid() != 0 {
				printWarning("需要root权限重载systemd服务")
				printInfo("请使用以下命令之一：")
				fmt.Println("  sudo ./agent reload")
				fmt.Println("  sudo systemctl reload cloudsentinel-agent")
				return fmt.Errorf("需要root权限")
			}

			// 使用systemd重载
			if err := systemd.ReloadService(); err != nil {
				printError(fmt.Sprintf("重载失败: %v", err))
				printInfo("使用以下命令查看详细错误信息：")
				fmt.Println("  sudo systemctl status cloudsentinel-agent")
				fmt.Println("  sudo journalctl -u cloudsentinel-agent -n 50")
				return err
			}

			printSuccess("agent已通过systemd服务重载配置")
			return nil
		}
	}

	// 直接启动模式
	pid, running, err := daemon.CheckPIDFile(pidFile)
	if err != nil {
		return fmt.Errorf("检查PID文件失败: %w", err)
	}

	if !running {
		printStatus("stopped", "agent未运行")
		return fmt.Errorf("agent未运行")
	}

	// 发送SIGHUP信号
	if err := daemon.SendSignal(pid, syscall.SIGHUP); err != nil {
		printError(fmt.Sprintf("发送重载信号失败: %v", err))
		return err
	}

	printSuccess(fmt.Sprintf("已发送重载信号 (PID: %d)", pid))
	return nil
}
