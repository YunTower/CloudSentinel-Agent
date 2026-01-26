package cli

import (
	"agent/internal/svc"
	"fmt"

	"github.com/spf13/cobra"
)

// stopCmd 停止命令
var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "停止服务",
	Long:  `停止CloudSentinel Agent服务。`,
	RunE:  runStop,
}

func init() {
	rootCmd.AddCommand(stopCmd)
}

func runStop(cmd *cobra.Command, args []string) error {
	s, err := svc.New(configPath)
	if err != nil {
		return fmt.Errorf("初始化服务配置失败: %w", err)
	}

	if err := s.Stop(); err != nil {
		return fmt.Errorf("停止服务失败: %w", err)
	}

	printSuccess("服务停止请求已发送")
	return nil
}
