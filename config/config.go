package config

import (
	"agent/internal/logger"
	"agent/internal/system"
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
)

type Config struct {
	Server string `json:"server"`
	Key    string `json:"key"`
	LogPath string `json:"log_path"`
}

func LoadConfig() Config {
	var cfg Config
	
	// 解析命令行参数
	serverFlag := flag.String("server", "", "WebSocket服务器地址 (例如: ws://127.0.0.1:3000/ws/agent)")
	keyFlag := flag.String("key", "", "Agent通信密钥")
	flag.Parse()
	
	// 获取配置文件路径（当前目录）
	configPath := "agent.lock.json"
	
	// 如果文件存在，读取配置
	_, err := os.Stat(configPath)
	if err == nil {
		file, err := os.ReadFile(configPath)
		if err != nil {
			fmt.Println("读取锁定文件时出错:", err)
			os.Exit(1)
		}
		
		err = json.Unmarshal(file, &cfg)
		if err != nil {
			fmt.Println("解析JSON数据时出错:", err)
			os.Exit(1)
		}
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
		fmt.Println("方式1: 使用命令行参数")
		fmt.Println("  agent.exe --server=ws://xxx.xxx.xxx.xxx/ws/agent --key=your-agent-key")
		fmt.Println("")
		fmt.Println("方式2: 交互式输入缺失的配置（将更新 agent.lock.json 配置文件）")
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
		
		// 创建或更新锁定文件（写入当前目录）
		file, err := os.Create("agent.lock.json")
		if err != nil {
			fmt.Println("创建文件时出错:", err)
			os.Exit(1)
		}
		defer file.Close()
		
		// 使用json格式写入配置
		configJSON, err := json.Marshal(cfg)
		if err != nil {
			fmt.Println("序列化配置时出错:", err)
			os.Exit(1)
		}
		_, err = file.Write(configJSON)
		if err != nil {
			fmt.Println("写入文件时出错:", err)
			os.Exit(1)
		}
		fmt.Println("配置已保存到 agent.lock.json")
	}

	if cfg.LogPath == "" {
		cfg.LogPath = "logs"
	}
	return cfg
}

func InitLogger(logPath string) *logger.Logger {
	logger, err := logger.NewLogger(logPath)
	if err != nil {
		fmt.Println("初始化日志时出错:", err)
		os.Exit(1)
	}
	return logger
}

func InitSystem() *system.System {
	return &system.System{}
}
