package cli

import (
	"agent/config"
	"fmt"

	"github.com/spf13/cobra"
)

// checkCmd 检查命令
var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "检查配置",
	Long:  `检查配置文件格式是否正确。`,
	RunE:  runCheck,
}

func init() {
	rootCmd.AddCommand(checkCmd)
}

func runCheck(cmd *cobra.Command, args []string) error {
	cfgPath := configPath
	if cfgPath == "" {
		cfgPath = config.GetConfigPath()
	}

	printInfo(fmt.Sprintf("正在检查配置文件: %s", cfgPath))

	cfg, err := config.LoadConfigFromFile(cfgPath)
	if err != nil {
		printError(fmt.Sprintf("配置加载失败: %v", err))
		return err
	}

	// 基础校验
	hasError := false
	if cfg.Server == "" {
		printError("Server 地址未配置")
		hasError = true
	}
	if cfg.Key == "" {
		printError("Key 未配置")
		hasError = true
	}

	if hasError {
		return fmt.Errorf("配置检查未通过")
	}

	printSuccess("配置格式正确")
	fmt.Printf("  Server: %s\n", cfg.Server)
	fmt.Printf("  LogPath: %s\n", cfg.LogPath)

	return nil
}
