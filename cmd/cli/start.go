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

// isRunningUnderSystemd 检查是否在systemd环境中运行
func isRunningUnderSystemd() bool {
	// systemd会设置INVOCATION_ID环境变量
	if os.Getenv("INVOCATION_ID") != "" {
		return true
	}
	// 检查父进程是否为systemd (PID 1)
	if os.Getppid() == 1 {
		return true
	}
	return false
}

func runStart(cmd *cobra.Command, args []string) error {
	// 如果systemd服务存在，且不是由systemd调用的，优先使用systemd管理
	if systemd.ServiceExists() && !isRunningUnderSystemd() {
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

		time.Sleep(200 * time.Millisecond)

		// 快速检查状态
		for i := 0; i < 3; i++ {
			active, err := systemd.IsServiceActive()
			if err == nil && active {
				printSuccess("agent已启动")
				return nil
			}
			if i < 2 {
				time.Sleep(200 * time.Millisecond)
			}
		}

		// 如果还没启动，给出提示但不阻塞
		printInfo("启动命令已执行，服务正在启动中")
		printInfo("使用 './agent status' 查看状态，'./agent logs' 查看日志")
		return nil
	}

	// 检查是否已经运行（直接启动模式）
	// 在systemd环境中，不检查PID文件，因为systemd会管理进程
	if !isRunningUnderSystemd() {
		pid, running, err := daemon.CheckPIDFile(pidFile)
		if err != nil {
			return fmt.Errorf("检查PID文件失败: %w", err)
		}
		if running {
			printStatus("running", fmt.Sprintf("agent已在运行中 (PID: %d)", pid))
			return nil
		}
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

	// 如果指定了守护进程模式（且不在systemd环境中）
	if daemonFlag && !isRunningUnderSystemd() {
		// 转换为守护进程
		if err := daemon.Daemonize(); err != nil {
			return fmt.Errorf("守护进程化失败: %w", err)
		}
	}

	// 写入PID文件（仅在非systemd环境中）
	if !isRunningUnderSystemd() {
		if err := daemon.WritePID(pidFile); err != nil {
			return fmt.Errorf("写入PID文件失败: %w", err)
		}
		defer daemon.RemovePID(pidFile)
	}

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
