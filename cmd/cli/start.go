package cli

import (
	"fmt"

	"agent/config"
	"agent/internal/agent"
	"agent/internal/daemon"

	"github.com/spf13/cobra"
)

var daemonFlag bool

// startCmd 启动命令
var startCmd = &cobra.Command{
	Use:   "start",
	Short: "启动agent",
	Long:  `启动CloudSentinel Agent。可以使用--daemon参数在后台运行。`,
	RunE:  runStart,
}

func init() {
	startCmd.Flags().BoolVarP(&daemonFlag, "daemon", "d", false, "以守护进程模式运行")
	rootCmd.AddCommand(startCmd)
}

func runStart(cmd *cobra.Command, args []string) error {
	// 检查是否已经运行
	pid, running, err := daemon.CheckPIDFile(pidFile)
	if err != nil {
		return fmt.Errorf("检查PID文件失败: %w", err)
	}
	if running {
		return fmt.Errorf("agent已经在运行中 (PID: %d)", pid)
	}

	// 加载配置
	cfgPath := configPath
	if cfgPath == "" {
		cfgPath = config.GetConfigPath()
	}

	cfg, err := config.LoadConfigFromFile(cfgPath)
	if err != nil {
		// 如果配置文件不存在，使用旧的LoadConfig方法
		cfg = config.LoadConfig()
	}

	// 如果指定了守护进程模式
	if daemonFlag {
		// 转换为守护进程
		if err := daemon.Daemonize(); err != nil {
			return fmt.Errorf("守护进程化失败: %w", err)
		}
	}

	// 写入PID文件
	if err := daemon.WritePID(pidFile); err != nil {
		return fmt.Errorf("写入PID文件失败: %w", err)
	}
	defer daemon.RemovePID(pidFile)

	// 创建并启动agent
	ag, err := agent.NewAgent(cfg)
	if err != nil {
		return fmt.Errorf("创建agent失败: %w", err)
	}

	if err := ag.Start(); err != nil {
		return fmt.Errorf("启动agent失败: %w", err)
	}

	// 等待退出
	ag.Wait()

	return nil
}
