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

// checkSystemdInstalled 检查 systemd 是否已安装
func checkSystemdInstalled() bool {
	// 检查 systemctl 命令是否存在
	if _, err := exec.LookPath("systemctl"); err != nil {
		return false
	}

	// 尝试执行 systemctl --version 来验证 systemd 是否可用
	cmd := exec.Command("systemctl", "--version")
	if err := cmd.Run(); err != nil {
		return false
	}

	return true
}

func runInstall(cmd *cobra.Command, args []string) error {
	// 检查是否为root用户
	if os.Geteuid() != 0 {
		return fmt.Errorf("安装systemd服务需要root权限，请使用sudo运行")
	}

	// 检查 systemd 是否已安装
	if !checkSystemdInstalled() {
		printWarning("未检测到 systemd")
		printInfo("请先安装 systemd：")
		fmt.Println("  Ubuntu/Debian: sudo apt-get install systemd")
		fmt.Println("  CentOS/RHEL:   sudo yum install systemd")
		fmt.Println("  Fedora:        sudo dnf install systemd")
		return fmt.Errorf("systemd 未安装")
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
	fmt.Println("  sudo systemctl start cloudsentinel-agent    # 启动服务")
	fmt.Println("  sudo systemctl stop cloudsentinel-agent     # 停止服务")
	fmt.Println("  sudo systemctl restart cloudsentinel-agent # 重启服务")
	fmt.Println("  sudo systemctl status cloudsentinel-agent  # 查看状态")
	fmt.Println("  sudo journalctl -u cloudsentinel-agent -f   # 查看日志")
	fmt.Println()
	printInfo("或使用 agent 命令:")
	fmt.Println("  sudo ./agent start    # 启动服务")
	fmt.Println("  sudo ./agent stop     # 停止服务")
	fmt.Println("  sudo ./agent restart  # 重启服务")
	fmt.Println("  ./agent status        # 查看状态")
	fmt.Println("  ./agent logs          # 查看日志")

	return nil
}
