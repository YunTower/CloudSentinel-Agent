package cli

import (
	"fmt"
	"os"
	"time"

	"agent/config"
	"agent/internal/agent"
	"agent/internal/daemon"
	"agent/internal/systemd"

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
	// 如果systemd服务存在，优先使用systemd管理
	if systemd.ServiceExists() {
		active, _ := systemd.IsServiceActive()
		if active {
			printStatus("running", "agent已在运行中")
			return nil
		}

		// 需要root权限操作systemd服务
		if os.Geteuid() != 0 {
			printWarning("需要root权限启动服务")
			printInfo("请使用: sudo ./agent start")
			return fmt.Errorf("需要root权限")
		}

		// 使用systemd启动
		if err := systemd.StartService(); err != nil {
			printError(fmt.Sprintf("启动失败: %v", err))
			printInfo("使用 './agent logs' 查看详细错误信息")
			return err
		}

		// 等待一小段时间，让服务启动
		time.Sleep(500 * time.Millisecond)

		// 再次检查状态，确认是否启动成功
		active, err := systemd.IsServiceActive()
		if err == nil && active {
			printSuccess("agent已启动")
		} else {
			printWarning("启动命令已执行，但服务可能未成功启动")
			printInfo("使用 './agent logs' 查看详细日志")
		}
		return nil
	}

	// 检查是否已经运行（直接启动模式）
	pid, running, err := daemon.CheckPIDFile(pidFile)
	if err != nil {
		return fmt.Errorf("检查PID文件失败: %w", err)
	}
	if running {
		printStatus("running", fmt.Sprintf("agent已在运行中 (PID: %d)", pid))
		return nil
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
