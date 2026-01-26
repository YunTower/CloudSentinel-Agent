package cli

import (
	"agent/internal/svc"
	"fmt"
	"os"
	"runtime"

	"github.com/spf13/cobra"
)

// installCmd 安装命令
var installCmd = &cobra.Command{
	Use:   "install",
	Short: "安装服务",
	Long:  `安装CloudSentinel Agent为系统服务。需要管理员/Root权限。`,
	RunE:  runInstall,
}

func init() {
	rootCmd.AddCommand(installCmd)
}

func runInstall(cmd *cobra.Command, args []string) error {
	// Linux下检查root权限
	if runtime.GOOS != "windows" && os.Geteuid() != 0 {
		return fmt.Errorf("安装服务需要root权限，请使用sudo运行")
	}

	s, err := svc.New(configPath)
	if err != nil {
		return fmt.Errorf("初始化服务配置失败: %w", err)
	}

	if err := s.Install(); err != nil {
		return fmt.Errorf("安装服务失败: %w", err)
	}

	printSuccess("服务安装成功")
	fmt.Println()
	printInfo("使用以下命令管理服务:")
	if runtime.GOOS == "windows" {
		fmt.Println("  agent start    # 启动服务")
		fmt.Println("  agent stop     # 停止服务")
		fmt.Println("  agent restart  # 重启服务")
		fmt.Println("  agent status   # 查看状态")
	} else {
		fmt.Println("  sudo agent start    # 启动服务")
		fmt.Println("  sudo agent stop     # 停止服务")
		fmt.Println("  sudo agent restart  # 重启服务")
		fmt.Println("  agent status        # 查看状态")
	}
	return nil
}
