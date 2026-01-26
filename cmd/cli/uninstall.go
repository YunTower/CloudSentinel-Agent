package cli

import (
	"agent/config"
	"agent/internal/svc"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"
)

var cleanLogs bool

// uninstallCmd 卸载命令
var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "卸载服务",
	Long:  `卸载CloudSentinel Agent系统服务。需要管理员/Root权限。`,
	RunE:  runUninstall,
}

func init() {
	uninstallCmd.Flags().BoolVar(&cleanLogs, "clean-logs", false, "同时删除日志文件")
	rootCmd.AddCommand(uninstallCmd)
}

func runUninstall(cmd *cobra.Command, args []string) error {
	// Linux下检查root权限
	if runtime.GOOS != "windows" && os.Geteuid() != 0 {
		return fmt.Errorf("卸载服务需要root权限，请使用sudo运行")
	}

	// 在卸载前先获取配置，以便后续清理日志
	cfg, cfgErr := config.LoadConfigFromFile(configPath)

	s, err := svc.New(configPath)
	if err != nil {
		return fmt.Errorf("初始化服务配置失败: %w", err)
	}

	// 尝试停止服务，但不报错
	_ = s.Stop()

	if err := s.Uninstall(); err != nil {
		return fmt.Errorf("卸载服务失败: %w", err)
	}

	printSuccess("服务卸载成功")

	// 卸载全局命令
	if err := removeGlobalCommand(); err != nil {
		printWarning(fmt.Sprintf("移除全局命令失败: %v", err))
	} else {
		printSuccess("全局命令已移除")
	}

	// 清除日志
	if cleanLogs {
		if cfgErr == nil {
			logPath := cfg.LogPath
			if !filepath.IsAbs(logPath) {
				// 如果是相对路径，转换为绝对路径
				if execPath, err := os.Executable(); err == nil {
					logPath = filepath.Join(filepath.Dir(execPath), logPath)
				}
			}

			if err := os.RemoveAll(logPath); err != nil {
				printWarning(fmt.Sprintf("清除日志失败: %v", err))
			} else {
				printSuccess(fmt.Sprintf("日志目录已清除: %s", logPath))
			}
		} else {
			printWarning("无法加载配置，跳过日志清理")
		}
	}

	return nil
}

func removeGlobalCommand() error {
	if runtime.GOOS == "windows" {
		return nil // Windows暂不处理全局命令移除，通常由安装程序处理
	}

	// 尝试移除常见的软链接位置
	paths := []string{
		"/usr/bin/cloudsentinel",
		"/usr/local/bin/cloudsentinel",
	}

	for _, p := range paths {
		if _, err := os.Lstat(p); err == nil {
			if err := os.Remove(p); err != nil {
				return fmt.Errorf("无法移除 %s: %w", p, err)
			}
		}
	}
	return nil
}
