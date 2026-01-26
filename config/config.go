package config

import (
	"agent/internal/logger"
	"agent/internal/system"
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Server            string `json:"server"`
	Key               string `json:"key"`
	LogPath           string `json:"log_path"`
	MetricsInterval   int    `json:"metrics_interval"`             // 性能指标上报间隔（秒）
	DetailInterval    int    `json:"detail_interval"`              // 详细信息上报间隔（秒）
	SystemInterval    int    `json:"system_interval"`              // 系统信息上报间隔（秒）
	HeartbeatInterval int    `json:"heartbeat_interval"`           // 心跳间隔（秒）
	Timezone          string `json:"timezone,omitempty"`           // 时区设置，默认 Asia/Shanghai
	AgentPrivateKey   string `json:"agent_private_key,omitempty"`  // Agent 私钥（PEM格式）
	AgentPublicKey    string `json:"agent_public_key,omitempty"`   // Agent 公钥（PEM格式）
	PanelPublicKey    string `json:"panel_public_key,omitempty"`   // 面板公钥（PEM格式）
	PanelFingerprint  string `json:"panel_fingerprint,omitempty"`  // 面板公钥指纹
	SessionKey        string `json:"session_key,omitempty"`        // AES 会话密钥（Base64编码字符串）
	EncryptionEnabled bool   `json:"encryption_enabled,omitempty"` // 是否启用加密
	LogRetentionDays  int    `json:"log_retention_days"`           // 日志保留天数
}

// LoadConfigFromFile 从指定文件加载配置
func LoadConfigFromFile(configPath string) (Config, error) {
	var cfg Config

	// 如果文件存在，读取配置
	_, err := os.Stat(configPath)
	if err == nil {
		file, err := os.ReadFile(configPath)
		if err != nil {
			return cfg, fmt.Errorf("读取配置文件时出错: %w", err)
		}

		err = json.Unmarshal(file, &cfg)
		if err != nil {
			return cfg, fmt.Errorf("解析JSON数据时出错: %w", err)
		}
	} else {
		return cfg, fmt.Errorf("配置文件不存在: %s", configPath)
	}

	// 设置默认值
	if cfg.LogPath == "" {
		cfg.LogPath = "logs"
	}

	// 设置默认上报间隔
	if cfg.MetricsInterval <= 0 {
		cfg.MetricsInterval = 5
	}
	if cfg.DetailInterval <= 0 {
		cfg.DetailInterval = 15
	}
	if cfg.SystemInterval <= 0 {
		cfg.SystemInterval = 15
	}
	if cfg.HeartbeatInterval <= 0 {
		cfg.HeartbeatInterval = 10
	}

	// 设置默认时区
	if cfg.Timezone == "" {
		cfg.Timezone = "Asia/Shanghai"
	}

	// 设置默认日志保留天数
	if cfg.LogRetentionDays <= 0 {
		cfg.LogRetentionDays = 7
	}

	return cfg, nil
}

// GetConfigPath 获取配置文件路径
func GetConfigPath() string {
	execPath, err := os.Executable()
	if err != nil {
		return "agent.lock.json"
	}
	execDir := ""
	if idx := len(execPath) - 1; idx >= 0 {
		for i := idx; i >= 0; i-- {
			if execPath[i] == '/' || execPath[i] == '\\' {
				execDir = execPath[:i]
				break
			}
		}
	}
	if execDir == "" {
		execDir = "."
	}
	return execDir + "/agent.lock.json"
}

// SaveConfig 保存配置到文件
func SaveConfig(cfg Config, configPath string) error {
	configJSON, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化配置时出错: %w", err)
	}

	err = os.WriteFile(configPath, configJSON, 0644)
	if err != nil {
		return fmt.Errorf("写入文件时出错: %w", err)
	}

	return nil
}

// SetConfigValue 设置配置项的值
func (c *Config) SetConfigValue(key, value string) error {
	switch key {
	case "server":
		c.Server = value
	case "key":
		c.Key = value
	case "log_path":
		c.LogPath = value
	case "metrics_interval":
		val, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("metrics_interval必须是整数: %w", err)
		}
		if val <= 0 {
			return fmt.Errorf("metrics_interval必须大于0")
		}
		c.MetricsInterval = val
	case "detail_interval":
		val, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("detail_interval必须是整数: %w", err)
		}
		if val <= 0 {
			return fmt.Errorf("detail_interval必须大于0")
		}
		c.DetailInterval = val
	case "system_interval":
		val, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("system_interval必须是整数: %w", err)
		}
		if val <= 0 {
			return fmt.Errorf("system_interval必须大于0")
		}
		c.SystemInterval = val
	case "heartbeat_interval":
		val, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("heartbeat_interval必须是整数: %w", err)
		}
		if val <= 0 {
			return fmt.Errorf("heartbeat_interval必须大于0")
		}
		c.HeartbeatInterval = val
	case "log_retention_days":
		val, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("log_retention_days必须是整数: %w", err)
		}
		if val <= 0 {
			return fmt.Errorf("log_retention_days必须大于0")
		}
		c.LogRetentionDays = val
	default:
		return fmt.Errorf("未知的配置项: %s", key)
	}
	return nil
}

// GetConfigValue 获取配置项的值
func (c *Config) GetConfigValue(key string) (string, error) {
	switch key {
	case "server":
		return c.Server, nil
	case "key":
		return c.Key, nil
	case "log_path":
		return c.LogPath, nil
	case "metrics_interval":
		return fmt.Sprintf("%d", c.MetricsInterval), nil
	case "detail_interval":
		return fmt.Sprintf("%d", c.DetailInterval), nil
	case "system_interval":
		return fmt.Sprintf("%d", c.SystemInterval), nil
	case "heartbeat_interval":
		return fmt.Sprintf("%d", c.HeartbeatInterval), nil
	case "log_retention_days":
		return fmt.Sprintf("%d", c.LogRetentionDays), nil
	default:
		return "", fmt.Errorf("未知的配置项: %s", key)
	}
}

func LoadConfig() Config {
	var cfg Config

	// 解析命令行参数
	serverFlag := flag.String("server", "", "WebSocket服务器地址 (例如: ws://127.0.0.1:3000/ws/agent)")
	keyFlag := flag.String("key", "", "Agent通信密钥")
	flag.Parse()

	// 获取配置文件路径（程序所在目录）
	configPath := GetConfigPath()

	// 尝试从文件加载配置
	fileCfg, err := LoadConfigFromFile(configPath)
	if err == nil {
		cfg = fileCfg
	}

	// 命令行参数优先于配置文件
	if *serverFlag != "" {
		cfg.Server = *serverFlag
	}
	if *keyFlag != "" {
		cfg.Key = *keyFlag
	}

	// 验证配置是否完整
	if cfg.Server == "" || cfg.Key == "" {
		missingFields := []string{}
		if cfg.Server == "" {
			missingFields = append(missingFields, "服务器地址")
		}
		if cfg.Key == "" {
			missingFields = append(missingFields, "通信密钥")
		}

		fmt.Printf("配置不完整，缺少: %s\n", strings.Join(missingFields, "、"))
		fmt.Println("")

		reader := bufio.NewReader(os.Stdin)

		if cfg.Server == "" {
			fmt.Print("主控WebSocket: ")
			input, err := reader.ReadString('\n')
			if err != nil {
				fmt.Println("读取输入时出错:", err)
				os.Exit(1)
			}
			cfg.Server = strings.TrimSpace(input)
		}

		if cfg.Key == "" {
			fmt.Print("通信密钥: ")
			input, err := reader.ReadString('\n')
			if err != nil {
				fmt.Println("读取输入时出错:", err)
				os.Exit(1)
			}
			cfg.Key = strings.TrimSpace(input)
		}

		// 验证输入
		if cfg.Server == "" || cfg.Key == "" {
			fmt.Println("服务器地址和通信密钥不能为空")
			os.Exit(1)
		}

		// 设置默认时区
		if cfg.Timezone == "" {
			cfg.Timezone = "Asia/Shanghai"
		}

		// 保存配置到文件
		if err := SaveConfig(cfg, configPath); err != nil {
			fmt.Printf("保存配置时出错: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("配置已保存到 %s\n", configPath)
	}

	// 如果未设置，则设置默认值
	if cfg.LogPath == "" {
		cfg.LogPath = "logs"
	}

	// 设置默认上报间隔
	if cfg.MetricsInterval <= 0 {
		cfg.MetricsInterval = 30 // 默认30秒
	}
	if cfg.DetailInterval <= 0 {
		cfg.DetailInterval = 30 // 默认30秒
	}
	if cfg.SystemInterval <= 0 {
		cfg.SystemInterval = 30 // 默认30秒
	}
	if cfg.HeartbeatInterval <= 0 {
		cfg.HeartbeatInterval = 20 // 默认20秒
	}

	// 设置默认时区
	if cfg.Timezone == "" {
		cfg.Timezone = "Asia/Shanghai"
	}

	return cfg
}

func InitLogger(logPath string, retentionDays int) *logger.Logger {
	logger, err := logger.NewLogger(logPath, retentionDays)
	if err != nil {
		fmt.Println("初始化日志时出错:", err)
		os.Exit(1)
	}
	return logger
}

func InitSystem() *system.System {
	return &system.System{}
}
