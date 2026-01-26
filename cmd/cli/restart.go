package cli

import (
	"agent/internal/svc"
	"fmt"

	"github.com/spf13/cobra"
)

// restartCmd 重启命令
var restartCmd = &cobra.Command{
	Use:   "restart",
	Short: "重启服务",
	Long:  `重启CloudSentinel Agent服务。`,
	RunE:  runRestart,
}

func init() {
	rootCmd.AddCommand(restartCmd)
}

func runRestart(cmd *cobra.Command, args []string) error {
	s, err := svc.New(configPath)
	if err != nil {
		return fmt.Errorf("初始化服务配置失败: %w", err)
	}

	if err := s.Restart(); err != nil {
		return fmt.Errorf("重启服务失败: %w", err)
	}

	printSuccess("服务重启请求已发送")
	return nil
}
