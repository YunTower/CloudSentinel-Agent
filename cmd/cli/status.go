package cli

import (
	"agent/internal/svc"
	"fmt"

	"github.com/spf13/cobra"
)

// statusCmd 状态命令
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "查看服务状态",
	Long:  `查看CloudSentinel Agent服务状态。`,
	RunE:  runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) error {
	s, err := svc.New(configPath)
	if err != nil {
		return fmt.Errorf("初始化服务配置失败: %w", err)
	}

	status, err := s.Status()
	if err != nil {
		return fmt.Errorf("获取状态失败: %w", err)
	}

	switch status {
	case "running":
		printStatus("running", "服务状态: 运行中")
	case "stopped":
		printStatus("stopped", "服务状态: 已停止")
	default:
		printStatus("unknown", fmt.Sprintf("服务状态: 未知 (%s)", status))
	}
	return nil
}
