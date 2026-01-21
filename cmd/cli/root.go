package cli

import (
	"agent/internal/version"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var (
	configPath string
	pidFile    string
)

// getDefaultPIDFile 获取默认 PID 文件路径
func getDefaultPIDFile() string {
	// 优先使用环境变量
	if envPIDFile := os.Getenv("CLOUDSENTINEL_AGENT_PIDFILE"); envPIDFile != "" {
		return envPIDFile
	}

	// 如果是 root 用户，尝试使用 /var/run
	if os.Geteuid() == 0 {
		return "/var/run/cloudsentinel-agent.pid"
	}

	// 非 root 用户，使用用户目录或临时目录
	homeDir, err := os.UserHomeDir()
	if err == nil {
		pidPath := filepath.Join(homeDir, ".cloudsentinel-agent.pid")
		return pidPath
	}

	// 如果无法获取用户目录，使用临时目录
	return filepath.Join(os.TempDir(), "cloudsentinel-agent.pid")
}

// rootCmd 根命令
var rootCmd = &cobra.Command{
	Use:     "agent",
	Short:   "CloudSentinel Agent",
	Long:    `CloudSentinel Agent - 云哨 (CloudSentinel) Agent端`,
	Version: version.AgentVersion,
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
	defaultPIDFile := getDefaultPIDFile()
	rootCmd.PersistentFlags().StringVarP(&pidFile, "pidfile", "p", defaultPIDFile, "PID文件路径")
}
