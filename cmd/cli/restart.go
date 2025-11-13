package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// restartCmd 重启命令
var restartCmd = &cobra.Command{
	Use:   "restart",
	Short: "重启agent",
	Long:  `重启CloudSentinel Agent（先停止，再启动）。`,
	RunE:  runRestart,
}

func init() {
	restartCmd.Flags().BoolVarP(&daemonFlag, "daemon", "d", false, "以守护进程模式运行")
	rootCmd.AddCommand(restartCmd)
}

func runRestart(cmd *cobra.Command, args []string) error {
	// 先停止
	stopCmd := &cobra.Command{}
	stopCmd.SetArgs([]string{})
	if err := runStop(stopCmd, []string{}); err != nil {
		// 如果停止失败，继续尝试启动
		fmt.Printf("停止agent时出现警告: %v\n", err)
	}

	// 再启动
	startCmd := &cobra.Command{}
	startCmd.SetArgs([]string{})
	return runStart(startCmd, []string{})
}
