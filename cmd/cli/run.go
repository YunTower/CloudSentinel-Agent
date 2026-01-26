package cli

import (
	"agent/internal/svc"

	"github.com/spf13/cobra"
)

// runCmd 运行命令（前台）
var runCmd = &cobra.Command{
	Use:   "run",
	Short: "前台运行agent",
	Long:  `在前台运行CloudSentinel Agent。通常由服务管理器调用，也可用于调试。`,
	RunE:  runRun,
}

func init() {
	rootCmd.AddCommand(runCmd)
}

func runRun(cmd *cobra.Command, args []string) error {
	// configPath 是 root.go 中定义的全局变量
	s, err := svc.New(configPath)
	if err != nil {
		return err
	}
	return s.Run()
}
