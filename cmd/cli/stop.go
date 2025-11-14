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
	// 如果systemd服务存在，优先使用systemd管理
	if systemd.ServiceExists() {
		active, _ := systemd.IsServiceActive()
		if !active {
			printStatus("stopped", "agent未运行")
			return nil
		}

		// 需要root权限操作systemd服务
		if os.Geteuid() != 0 {
			printWarning("需要root权限停止服务")
			printInfo("请使用: sudo ./agent stop")
			return fmt.Errorf("需要root权限")
		}

		// 使用systemd停止
		if err := systemd.StopService(); err != nil {
			printError(fmt.Sprintf("停止失败: %v", err))
			return err
		}

		printSuccess("agent已停止")
		return nil
	}

	// 直接启动模式：读取PID
	pid, running, err := daemon.CheckPIDFile(pidFile)
	if err != nil {
		return fmt.Errorf("检查PID文件失败: %w", err)
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

	// 等待进程退出（最多等待10秒）
	for i := 0; i < 10; i++ {
		if !daemon.IsProcessRunning(pid) {
			printSuccess("agent已停止")
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
		printWarning("agent已强制终止")
		daemon.RemovePID(pidFile)
	}

	return nil
}
