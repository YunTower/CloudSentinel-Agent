package cli

import (
	"agent/internal/svc"
	"fmt"

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
		return fmt.Errorf("启动服务失败: %w\n请检查服务是否已安装 (agent install) 或是否有足够权限", err)
	}

	printSuccess("服务启动请求已发送")
	return nil
}
