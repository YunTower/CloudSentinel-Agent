package reporter

import (
	"agent/config"
	"agent/internal/crypto"
	"agent/internal/logger"
	"agent/internal/websocket"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"
)

type CpuInfo struct {
	Id         int    `json:"id"`
	Name       string `json:"name"`
	LogicCount int    `json:"logic_count"`
	Count      int    `json:"count"`
}

type MemoryIo struct {
	Total       int `json:"total"`
	Used        int `json:"used"`
	Free        int `json:"free"`
	UsedPercent int `json:"used_percent"`
}

type DiskIo struct {
	Name       string  `json:"name"`
	ReadCount  uint64  `json:"read_count"`
	WriteCount float64 `json:"write_count"`
	ReadBytes  float64 `json:"read_bytes"`
	WriteBytes float64 `json:"write_bytes"`
	ReadTime   int     `json:"read_time"`
	WriteTime  int     `json:"write_time"`
}

// sendAuthMessage 发送认证消息
func sendAuthMessage(client *websocket.Client, cfg *config.Config, logger *logger.Logger) {
	// 验证key是否存在
	if cfg.Key == "" {
		logger.Error("agent key为空，无法发送认证消息")
		return
	}

	// 生成或加载 Agent 密钥对
	var agentPublicKey string
	if cfg.AgentPublicKey != "" {
		// 使用已有的公钥
		agentPublicKey = cfg.AgentPublicKey
	} else {
		// 生成新的密钥对
		privateKeyBytes, publicKeyBytes, err := crypto.GenerateKeyPair()
		if err != nil {
			logger.Error("生成Agent密钥对失败: %v", err)
			// 密钥生成失败不影响认证，继续使用明文通信
		} else {
			cfg.AgentPrivateKey = string(privateKeyBytes)
			cfg.AgentPublicKey = string(publicKeyBytes)
			agentPublicKey = cfg.AgentPublicKey

			// 保存配置
			configPath := config.GetConfigPath()
			if err := config.SaveConfig(*cfg, configPath); err != nil {
				logger.Warn("保存Agent密钥对失败: %v", err)
			}
		}
	}

	authData := map[string]interface{}{
		"type": "server",
		"key":  cfg.Key,
	}

	// 如果生成了公钥，添加到认证数据中
	if agentPublicKey != "" {
		authData["agent_public_key"] = agentPublicKey
	}

	authMessage := websocket.Message{
		Type: "auth",
		Data: authData,
	}

	// 验证 key 长度
	if len(cfg.Key) != 36 {
		logger.Warn("警告: agent key 长度异常 (%d)，正常应该是 36 个字符", len(cfg.Key))
	}

	if err := client.SendMessage(authMessage); err != nil {
		logger.Error("发送认证消息失败: %v", err)
	}
}

// ReporterCallbacks 定义回调函数接口
type ReporterCallbacks struct {
	OnAuthSuccess func() // 认证成功时调用
	OnDisconnect  func() // 断开连接时调用
}

// StartReporter 启动消息处理循环，只负责消息读取和认证
func StartReporter(client *websocket.Client, logger *logger.Logger, cfg config.Config, callbacks ReporterCallbacks) {
	// 使用指针以便修改配置
	cfgPtr := &cfg

	// 连接成功后立即发送认证消息
	sendAuthMessage(client, cfgPtr, logger)

	// 消息读取循环
	for {
		// 检查是否已停止
		if client.IsStopped() {
			logger.Info("Reporter已停止")
			return
		}

		conn := client.GetConnection()
		if conn == nil {
			// 检查是否已停止
			if client.IsStopped() {
				logger.Info("Reporter已停止")
				return
			}
			logger.Error("连接不可用，尝试重连...")
			if err := client.Reconnect(); err != nil {
				logger.Error("重连失败: %v", err)
				// 等待期间定期检查停止状态
				for i := 0; i < 5; i++ {
					if client.IsStopped() {
						logger.Info("Reporter已停止")
						return
					}
					time.Sleep(1 * time.Second)
				}
				continue
			}
			conn = client.GetConnection()
			// 重连成功后立即发送认证消息
			sendAuthMessage(client, cfgPtr, logger)
			// 通知断开连接，让主进程重启子进程
			if callbacks.OnDisconnect != nil {
				callbacks.OnDisconnect()
			}
		}

		// 设置读取超时，防止阻塞
		conn.SetReadDeadline(time.Now().Add(90 * time.Second))

		// 读取消息（支持加密）
		var message []byte
		var err error
		if client.IsEncryptionEnabled() {
			message, err = client.ReadEncryptedMessage()
		} else {
			_, message, err = conn.ReadMessage()
		}
		if err != nil {
			// 检查是否已停止
			if client.IsStopped() {
				logger.Info("Reporter已停止")
				return
			}

			if err == io.EOF {
				logger.Warn("连接已关闭")
			} else {
				logger.Error("读取消息时出错: %v", err)
			}

			client.IsConnected = false

			if err := client.Reconnect(); err != nil {
				logger.Error("重连失败: %v", err)
				logger.Error("已达最大重连次数，请检查网络连接或后端服务状态")
				// 等待期间定期检查停止状态（每5秒检查一次，共60秒）
				for i := 0; i < 12; i++ {
					if client.IsStopped() {
						logger.Info("Reporter已停止")
						return
					}
					time.Sleep(5 * time.Second)
				}
				// 重置连接状态，允许下一轮重连
				continue
			} else {
				// 重连成功后立即发送认证消息
				sendAuthMessage(client, cfgPtr, logger)
				// 通知断开连接，让主进程重启子进程
				if callbacks.OnDisconnect != nil {
					callbacks.OnDisconnect()
				}
			}
			continue
		}

		var jsonData map[string]interface{}
		err = json.Unmarshal(message, &jsonData)
		if err != nil {
			logger.Error("解析JSON数据时出错: %v", err)
			continue
		}

		typeValue, _ := jsonData["type"].(string)
		statusValue, statusExists := jsonData["status"].(string)
		messageValue, messageExists := jsonData["message"].(string)

		// 处理密钥交换消息
		if typeValue == "key_exchange" && statusValue == "success" {
			if err := handleKeyExchange(jsonData, client, cfgPtr, logger); err != nil {
				logger.Error("密钥交换失败: %v", err)
			}
		}

		// 处理会话密钥消息
		if typeValue == "session_key" && statusValue == "success" {
			if err := handleSessionKey(jsonData, client, cfgPtr, logger); err != nil {
				logger.Error("接收会话密钥失败: %v", err)
			}
		}

		// 处理认证成功
		if statusExists && typeValue == "auth" && statusValue == "success" {
			logger.Success("认证成功")

			// 通知主进程认证成功，启动数据上报和心跳
			if callbacks.OnAuthSuccess != nil {
				callbacks.OnAuthSuccess()
			}
		}

		// 处理带状态和消息的响应
		if statusExists && messageExists {
			if statusValue != "success" {
				logger.Warn("%s: %s", typeValue, messageValue)
			}
		} else {
			// 处理服务器请求
			if !statusExists {
				switch typeValue {
				case "command":
					// 处理服务器命令
					commandData, ok := jsonData["command"].(string)
					if ok {
						if commandData == "restart" {
							logger.Info("收到重启命令，准备重启...")
							// 发送确认消息
							response := websocket.Message{
								Type: "command_response",
								Data: map[string]interface{}{
									"command": "restart",
									"status":  "success",
									"message": "正在重启...",
								},
							}
							if err := client.SendMessage(response); err != nil {
								logger.Error("发送重启确认消息失败: %v", err)
							}
							// 延迟一小段时间确保消息发送成功
							time.Sleep(500 * time.Millisecond)
							// TODO: 退出程序，由系统服务管理器（如systemd）自动重启
							logger.Info("退出程序，等待系统服务管理器重启...")
							os.Exit(0)
						} else if commandData == "update" {
							// 处理更新命令
							updateData, ok := jsonData["data"].(map[string]interface{})
							if !ok {
								logger.Error("更新命令数据格式错误")
								response := websocket.Message{
									Type: "command_response",
									Data: map[string]interface{}{
										"command": "update",
										"status":  "error",
										"message": "更新命令数据格式错误",
									},
								}
								client.SendMessage(response)
								continue
							}

							version, _ := updateData["version"].(string)
							versionType, _ := updateData["version_type"].(string)

							if version == "" {
								logger.Error("更新命令缺少版本号")
								response := websocket.Message{
									Type: "command_response",
									Data: map[string]interface{}{
										"command": "update",
										"status":  "error",
										"message": "更新命令缺少版本号",
									},
								}
								client.SendMessage(response)
								continue
							}

							logger.Info("收到更新命令，版本: %s, 类型: %s", version, versionType)

							// 发送确认消息
							response := websocket.Message{
								Type: "command_response",
								Data: map[string]interface{}{
									"command": "update",
									"status":  "success",
									"message": "开始更新...",
								},
							}
							if err := client.SendMessage(response); err != nil {
								logger.Error("发送更新确认消息失败: %v", err)
							}

							go func() {
								updateService := NewUpdateService(logger)
								if err := updateService.UpdateAgent(version, versionType); err != nil {
									logger.Error("更新失败: %v", err)
									// 发送错误响应
									errorResponse := websocket.Message{
										Type: "command_response",
										Data: map[string]interface{}{
											"command": "update",
											"status":  "error",
											"message": fmt.Sprintf("更新失败: %v", err),
										},
									}
									client.SendMessage(errorResponse)
								} else {
									// 发送成功响应
									successResponse := websocket.Message{
										Type: "command_response",
										Data: map[string]interface{}{
											"command": "update",
											"status":  "success",
											"message": "更新完成，正在重启...",
										},
									}
									client.SendMessage(successResponse)
								}
							}()
						} else {
							logger.Warn("未知的命令: %s", commandData)
						}
					}
				case "auth":
					// 服务器要求认证
					sendAuthMessage(client, cfgPtr, logger)
				default:
					logger.Warn("未知的消息类型: %v", typeValue)
				}
			}
		}
	}
}

// handleKeyExchange 处理密钥交换消息
func handleKeyExchange(jsonData map[string]interface{}, client *websocket.Client, cfg *config.Config, logger *logger.Logger) error {
	data, ok := jsonData["data"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("密钥交换数据格式错误")
	}

	panelPublicKey, ok := data["panel_public_key"].(string)
	if !ok || panelPublicKey == "" {
		return fmt.Errorf("缺少面板公钥")
	}

	panelFingerprint, ok := data["panel_fingerprint"].(string)
	if !ok || panelFingerprint == "" {
		return fmt.Errorf("缺少面板公钥指纹")
	}

	// 验证面板指纹
	if cfg.PanelFingerprint != "" {
		if cfg.PanelFingerprint != panelFingerprint {
			return fmt.Errorf("面板公钥指纹不匹配，可能存在中间人攻击")
		}
	} else {
		// 首次连接，保存指纹
		cfg.PanelFingerprint = panelFingerprint
	}

	// 计算接收到的面板公钥指纹并验证
	receivedFingerprint, err := crypto.GetPublicKeyFingerprint([]byte(panelPublicKey))
	if err != nil {
		return fmt.Errorf("计算面板公钥指纹失败: %w", err)
	}

	if receivedFingerprint != panelFingerprint {
		return fmt.Errorf("面板公钥指纹验证失败")
	}

	// 保存面板公钥
	cfg.PanelPublicKey = panelPublicKey

	// 保存配置
	configPath := config.GetConfigPath()
	if err := config.SaveConfig(*cfg, configPath); err != nil {
		logger.Warn("保存面板公钥失败: %v", err)
	}

	logger.Info("密钥交换成功，已接收面板公钥")

	return nil
}

// handleSessionKey 处理会话密钥消息
func handleSessionKey(jsonData map[string]interface{}, client *websocket.Client, cfg *config.Config, logger *logger.Logger) error {
	// 检查是否有Agent私钥
	if cfg.AgentPrivateKey == "" {
		return fmt.Errorf("缺少Agent私钥，无法解密会话密钥")
	}

	data, ok := jsonData["data"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("会话密钥数据格式错误")
	}

	encryptedSessionKeyBase64, ok := data["encrypted_session_key"].(string)
	if !ok || encryptedSessionKeyBase64 == "" {
		return fmt.Errorf("缺少加密的会话密钥")
	}

	// Base64 解码
	encryptedSessionKey, err := base64.StdEncoding.DecodeString(encryptedSessionKeyBase64)
	if err != nil {
		return fmt.Errorf("Base64解码失败: %w", err)
	}

	// 使用Agent私钥解密会话密钥
	sessionKey, err := crypto.DecryptWithPrivateKey(encryptedSessionKey, []byte(cfg.AgentPrivateKey))
	if err != nil {
		return fmt.Errorf("解密会话密钥失败: %w", err)
	}

	// 启用加密
	client.EnableEncryption(sessionKey)
	cfg.SessionKey = base64.StdEncoding.EncodeToString(sessionKey)
	cfg.EncryptionEnabled = true

	// 保存配置
	configPath := config.GetConfigPath()
	if err := config.SaveConfig(*cfg, configPath); err != nil {
		logger.Warn("保存会话密钥失败: %v", err)
	}

	logger.Success("会话密钥接收成功，加密通信已启用")

	return nil
}
