package cli

import (
	"agent/internal/svc"
	"fmt"

	"github.com/spf13/cobra"
)

// reloadCmd 重载命令
var reloadCmd = &cobra.Command{
	Use:   "reload",
	Short: "重载配置",
	Long:  `重启服务以应用新的配置。`,
	RunE:  runReload,
}

func init() {
	rootCmd.AddCommand(reloadCmd)
}

func runReload(cmd *cobra.Command, args []string) error {
	s, err := svc.New(configPath)
	if err != nil {
		return err
	}

	status, err := s.Status()
	if err != nil {
		return fmt.Errorf("获取服务状态失败: %w", err)
	}

	if status != "running" {
		printStatus("stopped", "agent未运行")
		return fmt.Errorf("agent未运行")
	}

	printInfo("正在重启服务以应用新配置...")
	if err := s.Restart(); err != nil {
		printError(fmt.Sprintf("服务重启失败: %v", err))
		return err
	}

	printSuccess("服务已重启，配置已应用")
	return nil
}
