package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	configPath string
	pidFile    string
)

// rootCmd 根命令
var rootCmd = &cobra.Command{
	Use:     "agent",
	Short:   "CloudSentinel Agent",
	Long:    `CloudSentinel Agent 是云哨 (CloudSentinel) 的Agent端，用于收集系统信息并上报到云哨控制面板。`,
	Version: "0.0.1",
}

// Execute 执行根命令
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		os.Exit(1)
	}
}

func init() {
	// 全局flag
	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "", "配置文件路径（默认：程序所在目录/agent.lock.json）")
	rootCmd.PersistentFlags().StringVarP(&pidFile, "pidfile", "p", "/var/run/cloudsentinel-agent.pid", "PID文件路径")
}
