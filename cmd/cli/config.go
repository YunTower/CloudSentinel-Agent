package cli

import (
	"encoding/json"
	"fmt"

	"agent/config"

	"github.com/spf13/cobra"
)

// configCmd 配置命令组
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "配置管理",
	Long:  `管理CloudSentinel Agent的配置。`,
}

// configSetCmd 设置配置项
var configSetCmd = &cobra.Command{
	Use:   "set [key] [value]",
	Short: "设置配置项",
	Long:  `设置配置项的值。支持的key: server, key, log_path, metrics_interval, detail_interval, system_interval, heartbeat_interval`,
	Args:  cobra.ExactArgs(2),
	RunE:  runConfigSet,
}

// configGetCmd 获取配置项
var configGetCmd = &cobra.Command{
	Use:   "get [key]",
	Short: "获取配置项",
	Long:  `获取配置项的值。支持的key: server, key, log_path, metrics_interval, detail_interval, system_interval, heartbeat_interval`,
	Args:  cobra.ExactArgs(1),
	RunE:  runConfigGet,
}

// configListCmd 列出所有配置
var configListCmd = &cobra.Command{
	Use:   "list",
	Short: "列出所有配置",
	Long:  `列出所有配置项及其值。`,
	RunE:  runConfigList,
}

func init() {
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configGetCmd)
	configCmd.AddCommand(configListCmd)
	rootCmd.AddCommand(configCmd)
}

func runConfigSet(cmd *cobra.Command, args []string) error {
	key := args[0]
	value := args[1]

	// 获取配置文件路径
	cfgPath := configPath
	if cfgPath == "" {
		cfgPath = config.GetConfigPath()
	}

	// 加载现有配置
	cfg, err := config.LoadConfigFromFile(cfgPath)
	if err != nil {
		// 如果文件不存在，创建新配置
		cfg = config.Config{}
	}

	// 设置配置值
	if err := cfg.SetConfigValue(key, value); err != nil {
		return err
	}

	// 保存配置
	if err := config.SaveConfig(cfg, cfgPath); err != nil {
		return fmt.Errorf("保存配置失败: %w", err)
	}

	printSuccess(fmt.Sprintf("配置项 %s 已设置为: %s", key, value))
	return nil
}

func runConfigGet(cmd *cobra.Command, args []string) error {
	key := args[0]

	// 获取配置文件路径
	cfgPath := configPath
	if cfgPath == "" {
		cfgPath = config.GetConfigPath()
	}

	// 加载配置
	cfg, err := config.LoadConfigFromFile(cfgPath)
	if err != nil {
		return fmt.Errorf("加载配置失败: %w", err)
	}

	// 获取配置值
	value, err := cfg.GetConfigValue(key)
	if err != nil {
		return err
	}

	fmt.Println(value)
	return nil
}

func runConfigList(cmd *cobra.Command, args []string) error {
	// 获取配置文件路径
	cfgPath := configPath
	if cfgPath == "" {
		cfgPath = config.GetConfigPath()
	}

	// 加载配置
	cfg, err := config.LoadConfigFromFile(cfgPath)
	if err != nil {
		return fmt.Errorf("加载配置失败: %w", err)
	}

	// 以JSON格式输出
	jsonData, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}

	fmt.Println(string(jsonData))
	return nil
}
