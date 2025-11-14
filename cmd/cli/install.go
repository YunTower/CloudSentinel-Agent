package cli

import (
	"fmt"
	"os"
	"os/exec"

	"agent/internal/systemd"

	"github.com/spf13/cobra"
)

// installCmd 安装命令
var installCmd = &cobra.Command{
	Use:   "install",
	Short: "安装systemd服务",
	Long:  `安装CloudSentinel Agent为systemd服务（需要root权限）。`,
	RunE:  runInstall,
}

func init() {
	rootCmd.AddCommand(installCmd)
}

func runInstall(cmd *cobra.Command, args []string) error {
	// 检查是否为root用户
	if os.Geteuid() != 0 {
		return fmt.Errorf("安装systemd服务需要root权限，请使用sudo运行")
	}

	// 获取可执行文件路径
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("获取可执行文件路径失败: %w", err)
	}

	// 获取绝对路径
	absPath, err := exec.LookPath(execPath)
	if err != nil {
		absPath = execPath
	}

	// 安装service文件
	if err := systemd.InstallService(absPath); err != nil {
		return fmt.Errorf("安装service文件失败: %w", err)
	}

	// 重新加载systemd daemon
	if err := systemd.ReloadDaemon(); err != nil {
		return fmt.Errorf("重新加载systemd daemon失败: %w", err)
	}

	// 启用服务（开机自启）
	if err := systemd.EnableService(); err != nil {
		return fmt.Errorf("启用服务失败: %w", err)
	}

	printSuccess("systemd服务安装成功")
	fmt.Println()
	printInfo("使用以下命令管理服务:")
	fmt.Println("  sudo ./agent start    # 启动服务")
	fmt.Println("  sudo ./agent stop     # 停止服务")
	fmt.Println("  sudo ./agent restart  # 重启服务")
	fmt.Println("  ./agent status        # 查看状态")
	fmt.Println("  ./agent logs          # 查看日志")

	return nil
}
