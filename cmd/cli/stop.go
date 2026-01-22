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
	// 优先检查systemd服务状态
	if systemd.ServiceExists() {
		active, err := systemd.IsServiceActive()
		if err != nil {
			// 检查systemd状态失败，继续尝试PID文件方式
			printWarning(fmt.Sprintf("检查systemd服务状态失败: %v，尝试使用PID文件方式", err))
		} else {
			if !active {
				printStatus("stopped", "agent未运行")
				return nil
			}

			// 需要root权限操作systemd服务
			if os.Geteuid() != 0 {
				printWarning("需要root权限停止systemd服务")
				printInfo("请使用以下命令之一：")
				fmt.Println("  sudo ./agent stop")
				fmt.Println("  sudo systemctl stop cloudsentinel-agent")
				return fmt.Errorf("需要root权限")
			}

			// 使用systemd停止
			if err := systemd.StopService(); err != nil {
				printError(fmt.Sprintf("停止失败: %v", err))
				printInfo("使用以下命令查看详细错误信息：")
				fmt.Println("  sudo systemctl status cloudsentinel-agent")
				fmt.Println("  sudo journalctl -u cloudsentinel-agent -n 50")
				return err
			}

			printSuccess("agent已通过systemd服务停止")
			return nil
		}
	}

	// 直接启动模式：读取PID
	pid, running, err := daemon.CheckPIDFile(pidFile)
	if err != nil {
		// PID文件检查失败，可能文件不存在或格式错误
		// 如果systemd服务也不存在，说明agent确实未运行
		printStatus("stopped", "agent未运行")
		// 清理可能残留的PID文件
		daemon.RemovePID(pidFile)
		return nil
	}

	if !running {
		printStatus("stopped", "agent未运行")
		// 清理可能残留的PID文件
		daemon.RemovePID(pidFile)
		return nil
	}

	// 发送SIGTERM信号
	if err := daemon.SendSignal(pid, syscall.SIGTERM); err != nil {
		return fmt.Errorf("发送停止信号失败: %w", err)
	}

	printInfo(fmt.Sprintf("已发送停止信号 (PID: %d)", pid))

	// 等待进程退出
	maxWait := 3 * time.Second
	checkInterval := 200 * time.Millisecond
	elapsed := time.Duration(0)

	for elapsed < maxWait {
		if !daemon.IsProcessRunning(pid) {
			printSuccess("agent已停止")
			daemon.RemovePID(pidFile)
			return nil
		}
		time.Sleep(checkInterval)
		elapsed += checkInterval
	}

	// 如果还在运行，尝试强制终止
	if daemon.IsProcessRunning(pid) {
		if err := daemon.SendSignal(pid, syscall.SIGKILL); err != nil {
			return fmt.Errorf("强制终止失败: %w", err)
		}
		printWarning("agent已强制终止")
		daemon.RemovePID(pidFile)
	}

	return nil
}
