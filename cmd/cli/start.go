package cli

import (
	"agent/internal/svc"
	"errors"
	"fmt"
	"runtime"
	"strings"

	"github.com/kardianos/service"
	"github.com/spf13/cobra"
)

// startCmd 启动命令
var startCmd = &cobra.Command{
	Use:   "start",
	Short: "启动服务",
	Long:  `启动CloudSentinel Agent服务。如果未安装服务，请先运行 'agent install'。`,
	RunE:  runStart,
}

func init() {
	rootCmd.AddCommand(startCmd)
}

func runStart(cmd *cobra.Command, args []string) error {
	s, err := svc.New(configPath)
	if err != nil {
		return fmt.Errorf("初始化服务配置失败: %w", err)
	}

	if err := s.Start(); err != nil {
		if errors.Is(err, service.ErrNotInstalled) {
			return fmt.Errorf("服务未安装，请先执行: agent install")
		}
		msg := err.Error()
		isPermissionErr := strings.Contains(msg, "permission") || strings.Contains(msg, "denied") ||
			strings.Contains(msg, "需要root") || strings.Contains(msg, "not permitted")
		if runtime.GOOS != "windows" && isPermissionErr {
			printWarning("启动 systemd 服务需要 root 权限")
			printInfo("请使用以下命令之一：")
			fmt.Println("  sudo cloudsentinel-agent start")
			fmt.Println("  sudo systemctl start cloudsentinel-agent")
			return fmt.Errorf("启动失败: %w", err)
		}
		return fmt.Errorf("启动服务失败: %w\n若因权限不足，请使用: sudo cloudsentinel-agent start 或 sudo systemctl start cloudsentinel-agent", err)
	}

	printSuccess("服务启动请求已发送")
	return nil
}
