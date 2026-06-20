package reporter

import (
	"agent/config"
	"agent/internal/crypto"
	"agent/internal/logger"
	"agent/internal/websocket"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
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

type pulledAgentTask struct {
	ID        string                 `json:"id"`
	Command   string                 `json:"command"`
	CommandID string                 `json:"command_id"`
	Data      map[string]interface{} `json:"data"`
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
	OnReload      func() // 重载配置时调用
}

// StartReporter 启动消息处理循环，只负责消息读取和认证
func StartReporter(client *websocket.Client, logger *logger.Logger, cfg config.Config, callbacks ReporterCallbacks) {
	// 使用指针以便修改配置
	cfgPtr := &cfg
	taskPollStarted := false

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

			// 发送当前配置到面板
			sendConfigToPanel(client, cfgPtr, logger)
			if !taskPollStarted {
				taskPollStarted = true
				go pollAgentTasks(client, cfgPtr, logger)
			}

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
						commandID, _ := jsonData["command_id"].(string)
						if commandData == "service_check" {
							sendCommandAck(client, commandData, commandID, logger)
							checkData, ok := jsonData["data"].(map[string]interface{})
							if ok {
								go handleServiceCheck(client, checkData, logger)
							}
						} else if commandData == "restart" {
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
							// 执行重启
							if err := restartAgent(logger); err != nil {
								logger.Error("重启失败: %v", err)
								os.Exit(1)
							}
							os.Exit(0)
						} else if commandData == "update_config" {
							// 处理配置更新命令
							updateData, ok := jsonData["data"].(map[string]interface{})
							if !ok {
								logger.Error("配置更新命令数据格式错误")
								response := websocket.Message{
									Type: "command_response",
									Data: map[string]interface{}{
										"command": "update_config",
										"status":  "error",
										"message": "配置更新命令数据格式错误",
									},
								}
								client.SendMessage(response)
								continue
							}

							// 更新配置
							configUpdated := false
							if timezone, ok := updateData["timezone"].(string); ok && timezone != "" {
								cfgPtr.Timezone = timezone
								configUpdated = true
							}
							if metricsInterval, ok := updateData["metrics_interval"].(float64); ok && metricsInterval > 0 {
								cfgPtr.MetricsInterval = int(metricsInterval)
								configUpdated = true
							}
							if detailInterval, ok := updateData["detail_interval"].(float64); ok && detailInterval > 0 {
								cfgPtr.DetailInterval = int(detailInterval)
								configUpdated = true
							}
							if systemInterval, ok := updateData["system_interval"].(float64); ok && systemInterval > 0 {
								cfgPtr.SystemInterval = int(systemInterval)
								configUpdated = true
							}
							if heartbeatInterval, ok := updateData["heartbeat_interval"].(float64); ok && heartbeatInterval > 0 {
								cfgPtr.HeartbeatInterval = int(heartbeatInterval)
								configUpdated = true
							}
							if logPath, ok := updateData["log_path"].(string); ok && logPath != "" {
								cfgPtr.LogPath = logPath
								configUpdated = true
							}

							if monitoredServices, ok := updateData["monitored_services"].([]interface{}); ok {
								var services []string
								for _, s := range monitoredServices {
									if str, ok := s.(string); ok {
										services = append(services, str)
									}
								}
								cfgPtr.MonitoredServices = services
								configUpdated = true
							}

							if configUpdated {
								// 保存配置到文件
								configPath := config.GetConfigPath()
								if err := config.SaveConfig(*cfgPtr, configPath); err != nil {
									logger.Error("保存配置失败: %v", err)
									response := websocket.Message{
										Type: "command_response",
										Data: map[string]interface{}{
											"command": "update_config",
											"status":  "error",
											"message": fmt.Sprintf("保存配置失败: %v", err),
										},
									}
									client.SendMessage(response)
									continue
								}

								logger.Info("配置已更新并保存")

								// 检查是否需要重启（server或key变化需要重启）
								needRestart := false
								if server, ok := updateData["server"].(string); ok && server != "" && server != cfgPtr.Server {
									needRestart = true
									logger.Info("检测到服务器地址变化，需要重启")
								}
								if key, ok := updateData["key"].(string); ok && key != "" && key != cfgPtr.Key {
									needRestart = true
									logger.Info("检测到密钥变化，需要重启")
								}

								if needRestart {
									// 需要重启，发送重启响应
									response := websocket.Message{
										Type: "command_response",
										Data: map[string]interface{}{
											"command": "update_config",
											"status":  "success",
											"message": "配置已更新，正在重启...",
										},
									}
									if err := client.SendMessage(response); err != nil {
										logger.Error("发送配置更新确认消息失败: %v", err)
									}
									// 延迟一小段时间确保消息发送成功
									time.Sleep(500 * time.Millisecond)
									// 执行重启
									if err := restartAgent(logger); err != nil {
										logger.Error("重启失败: %v", err)
										os.Exit(1)
									}
									os.Exit(0)
								} else {
									// 不需要重启，调用重载回调
									if callbacks.OnReload != nil {
										callbacks.OnReload()
									}

									response := websocket.Message{
										Type: "command_response",
										Data: map[string]interface{}{
											"command": "update_config",
											"status":  "success",
											"message": "配置已更新并重载",
										},
									}
									if err := client.SendMessage(response); err != nil {
										logger.Error("发送配置更新确认消息失败: %v", err)
									}
								}
							} else {
								logger.Warn("配置更新命令未包含有效配置项")
								response := websocket.Message{
									Type: "command_response",
									Data: map[string]interface{}{
										"command": "update_config",
										"status":  "success",
										"message": "未检测到配置变更",
									},
								}
								client.SendMessage(response)
							}
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

func sendCommandAck(client *websocket.Client, command, commandID string, logger *logger.Logger) {
	if commandID == "" {
		return
	}
	if err := client.SendMessage(websocket.Message{
		Type: "command_ack",
		Data: map[string]interface{}{
			"command":    command,
			"command_id": commandID,
			"status":     "received",
		},
	}); err != nil {
		logger.Warn("发送命令ACK失败: command=%s command_id=%s error=%v", command, commandID, err)
	}
}

func pollAgentTasks(client *websocket.Client, cfg *config.Config, logger *logger.Logger) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		if client.IsStopped() {
			return
		}
		if err := pullAndHandleAgentTasks(cfg, logger); err != nil {
			logger.Warn("拉取 Agent 任务失败: %v", err)
		}
		<-ticker.C
	}
}

func pullAndHandleAgentTasks(cfg *config.Config, logger *logger.Logger) error {
	endpoint, err := agentAPIEndpoint(cfg.Server, "/api/agent/tasks/pull")
	if err != nil {
		return err
	}
	var resp struct {
		Status bool              `json:"status"`
		Data   []pulledAgentTask `json:"data"`
	}
	if err := postAgentJSON(endpoint, map[string]interface{}{
		"agent_key": cfg.Key,
		"limit":     10,
	}, &resp); err != nil {
		return err
	}
	if !resp.Status || len(resp.Data) == 0 {
		return nil
	}

	for _, task := range resp.Data {
		status := "succeeded"
		errText := ""
		switch task.Command {
		case "service_check":
			result := performServiceCheck(task.Data, logger)
			if err := postAgentReport(cfg.Server, cfg.Key, "service_check_result", result); err != nil {
				status = "failed"
				errText = err.Error()
			}
		default:
			status = "failed"
			errText = fmt.Sprintf("unsupported task command: %s", task.Command)
		}
		if err := completeAgentTask(cfg.Server, cfg.Key, task.ID, status, errText); err != nil {
			logger.Warn("标记 Agent 任务完成失败: task_id=%s error=%v", task.ID, err)
		}
	}
	return nil
}

func postAgentReport(server, key, reportType string, data interface{}) error {
	endpoint, err := agentAPIEndpoint(server, "/api/agent/report")
	if err != nil {
		return err
	}
	return postAgentJSON(endpoint, map[string]interface{}{
		"agent_key": key,
		"type":      reportType,
		"data":      data,
	}, nil)
}

func completeAgentTask(server, key, taskID, status, errText string) error {
	endpoint, err := agentAPIEndpoint(server, "/api/agent/tasks/complete")
	if err != nil {
		return err
	}
	return postAgentJSON(endpoint, map[string]interface{}{
		"agent_key": key,
		"task_id":   taskID,
		"status":    status,
		"error":     errText,
	}, nil)
}

func postAgentJSON(endpoint string, payload interface{}, out interface{}) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(endpoint, "application/json", strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}

func agentAPIEndpoint(server, apiPath string) (string, error) {
	u, err := url.Parse(server)
	if err != nil {
		return "", err
	}
	switch u.Scheme {
	case "ws":
		u.Scheme = "http"
	case "wss":
		u.Scheme = "https"
	case "http", "https":
	default:
		return "", fmt.Errorf("unsupported server scheme: %s", u.Scheme)
	}
	basePath := strings.TrimRight(u.Path, "/")
	if strings.HasSuffix(basePath, "/ws/agent") {
		basePath = strings.TrimSuffix(basePath, "/ws/agent")
	}
	if strings.HasSuffix(basePath, "/api") {
		u.Path = strings.TrimRight(basePath, "/") + strings.TrimPrefix(apiPath, "/api")
	} else {
		u.Path = apiPath
	}
	u.RawQuery = ""
	u.Fragment = ""
	return u.String(), nil
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

// sendConfigToPanel 发送当前配置到面板
func sendConfigToPanel(client *websocket.Client, cfg *config.Config, logger *logger.Logger) {
	configMessage := websocket.Message{
		Type: "agent_config",
		Data: map[string]interface{}{
			"timezone":           cfg.Timezone,
			"metrics_interval":   cfg.MetricsInterval,
			"detail_interval":    cfg.DetailInterval,
			"system_interval":    cfg.SystemInterval,
			"heartbeat_interval": cfg.HeartbeatInterval,
			"log_path":           cfg.LogPath,
			"monitored_services": cfg.MonitoredServices,
		},
	}

	if err := client.SendMessage(configMessage); err != nil {
		logger.Error("发送配置到面板失败: %v", err)
	} else {
		logger.Info("已发送配置到面板")
	}
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

// restartAgent 重启agent程序
func restartAgent(logger *logger.Logger) error {
	// 获取当前可执行文件路径
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("获取可执行文件路径失败: %w", err)
	}

	// 尝试调用 CLI 的 restart 命令 (这会尝试重启服务)
	logger.Info("尝试通过 CLI 重启服务...")
	// 在 Windows 下，exec.Command 可能需要 .exe 后缀，os.Executable 通常包含它
	restartCmd := exec.Command(execPath, "restart")
	if err := restartCmd.Run(); err == nil {
		logger.Info("服务重启命令已发送")
		return nil
	}

	// 如果失败，回退到进程重启模式
	logger.Info("服务重启失败（可能未安装服务），切换到进程重启模式...")

	// 获取当前进程的PID
	pid := os.Getpid()

	// 构建重启命令（避免 shell 拼接，避免路径包含特殊字符导致的命令解释风险）
	cmd := exec.Command(execPath, "run")

	// 设置工作目录
	cmd.Dir = filepath.Dir(execPath)

	// 固定延迟，确保旧进程有时间退出/释放资源（避免使用 env/shell）
	time.Sleep(config.RestartStartDelay)

	// 启动新进程
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动新进程失败: %w", err)
	}

	logger.Info("新进程已启动，PID: %d，正在终止当前进程 PID: %d", cmd.Process.Pid, pid)

	// 等待新进程启动
	if runtime.GOOS == "windows" {
		time.Sleep(3 * time.Second)
	} else {
		time.Sleep(2 * time.Second)
	}

	return nil
}

// 处理服务检查
func handleServiceCheck(client *websocket.Client, data map[string]interface{}, logger *logger.Logger) {
	result := performServiceCheck(data, logger)
	_ = client.SendMessage(websocket.Message{
		Type: "service_check_result",
		Data: result,
	})
}

func performServiceCheck(data map[string]interface{}, logger *logger.Logger) map[string]interface{} {
	monitorID, _ := data["monitor_id"].(float64)
	checkID, _ := data["check_id"].(string)
	typ, _ := data["type"].(string)
	target, _ := data["target"].(string)
	port, _ := data["port"].(float64)
	timeout, _ := data["timeout"].(float64)
	expectStatus, _ := data["expect_status"].(float64)
	expectBody, _ := data["expect_body"].(string)
	httpMethod, _ := data["http_method"].(string)
	httpHeaders, _ := data["http_headers"].(string)
	httpBody, _ := data["http_body"].(string)

	if timeout <= 0 {
		timeout = 10
	}

	start := time.Now()
	var checkErr error

	switch typ {
	case "http", "https":
		checkErr = agentCheckHTTP(target, int(timeout), int(expectStatus), expectBody, httpMethod, httpHeaders, httpBody)
	case "tcp":
		checkErr = agentCheckTCP(target, int(port), int(timeout))
	case "udp":
		checkErr = agentCheckUDP(target, int(port), int(timeout))
	case "icmp", "ping":
		checkErr = agentCheckICMP(target, int(timeout))
	case "dns":
		checkErr = agentCheckDNS(target, int(timeout))
	case "tls":
		checkErr = agentCheckTLS(target, int(port), int(timeout))
	default:
		checkErr = fmt.Errorf("unknown type: %s", typ)
	}

	elapsed := int(time.Since(start).Milliseconds())
	status := "up"
	errText := ""
	if checkErr != nil {
		status = "down"
		errText = checkErr.Error()
		logger.Warn("服务检测 [%d] 失败: %v", int(monitorID), checkErr)
	}

	return map[string]interface{}{
		"monitor_id":    monitorID,
		"check_id":      checkID,
		"status":        status,
		"response_time": elapsed,
		"error":         errText,
	}
}

// 检查HTTP服务
func agentCheckHTTP(target string, timeoutSec, expectStatus int, expectBody, method, headersJSON, requestBody string) error {
	c := &http.Client{
		Timeout: time.Duration(timeoutSec) * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: false},
		},
	}
	method = normalizeRequestMethod(method)
	var body io.Reader
	if requestBody != "" && methodAllowsBody(method) {
		body = strings.NewReader(requestBody)
	}
	req, err := http.NewRequest(method, target, body)
	if err != nil {
		return err
	}
	headers, err := parseHTTPHeaders(headersJSON)
	if err != nil {
		return err
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	if requestBody != "" && methodAllowsBody(method) && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if expectStatus > 0 {
		if resp.StatusCode != expectStatus {
			return fmt.Errorf("期望状态码 %d，实际 %d", expectStatus, resp.StatusCode)
		}
	} else if resp.StatusCode >= 500 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	if expectBody != "" {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		if !strings.Contains(string(body), expectBody) {
			return fmt.Errorf("响应体不包含期望内容")
		}
	}
	return nil
}

func normalizeRequestMethod(method string) string {
	switch strings.ToUpper(strings.TrimSpace(method)) {
	case "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS":
		return strings.ToUpper(strings.TrimSpace(method))
	default:
		return "GET"
	}
}

func methodAllowsBody(method string) bool {
	switch method {
	case "POST", "PUT", "PATCH", "DELETE":
		return true
	default:
		return false
	}
}

func parseHTTPHeaders(raw string) (map[string]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return map[string]string{}, nil
	}
	headers := map[string]string{}
	if err := json.Unmarshal([]byte(raw), &headers); err != nil {
		return nil, fmt.Errorf("http headers must be a JSON object: %w", err)
	}
	for key := range headers {
		if strings.TrimSpace(key) == "" {
			return nil, fmt.Errorf("http header name cannot be empty")
		}
	}
	return headers, nil
}

// 检查TCP服务
func agentCheckTCP(host string, port, timeoutSec int) error {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", host, port), time.Duration(timeoutSec)*time.Second)
	if err != nil {
		return err
	}
	conn.Close()
	return nil
}

// 检查UDP服务
func agentCheckUDP(host string, port, timeoutSec int) error {
	conn, err := net.DialTimeout("udp", fmt.Sprintf("%s:%d", host, port), time.Duration(timeoutSec)*time.Second)
	if err != nil {
		return err
	}
	conn.Close()
	return nil
}

func agentCheckICMP(target string, timeoutSec int) error {
	if timeoutSec <= 0 {
		timeoutSec = 10
	}
	host, _, err := splitMonitorTarget(target, 0)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSec)*time.Second)
	defer cancel()

	args := []string{"-c", "1", "-W", fmt.Sprintf("%d", timeoutSec), host}
	if runtime.GOOS == "windows" {
		args = []string{"-n", "1", "-w", fmt.Sprintf("%d", timeoutSec*1000), host}
	}
	cmd := exec.CommandContext(ctx, "ping", args...)
	output, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return fmt.Errorf("ping timeout")
	}
	if err != nil {
		msg := strings.TrimSpace(string(output))
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("ping failed: %s", msg)
	}
	return nil
}

func agentCheckDNS(target string, timeoutSec int) error {
	if timeoutSec <= 0 {
		timeoutSec = 10
	}
	host, _, err := splitMonitorTarget(target, 0)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSec)*time.Second)
	defer cancel()

	ips, err := net.DefaultResolver.LookupHost(ctx, host)
	if err != nil {
		return fmt.Errorf("dns lookup failed: %w", err)
	}
	if len(ips) == 0 {
		return fmt.Errorf("dns lookup returned no records")
	}
	return nil
}

func agentCheckTLS(target string, port, timeoutSec int) error {
	if timeoutSec <= 0 {
		timeoutSec = 10
	}
	host, resolvedPort, err := splitMonitorTarget(target, 443)
	if err != nil {
		return err
	}
	if port > 0 {
		resolvedPort = port
	}

	dialer := &net.Dialer{Timeout: time.Duration(timeoutSec) * time.Second}
	conn, err := tls.DialWithDialer(dialer, "tcp", fmt.Sprintf("%s:%d", host, resolvedPort), &tls.Config{
		ServerName: host,
		MinVersion: tls.VersionTLS12,
	})
	if err != nil {
		return fmt.Errorf("tls connection failed: %w", err)
	}
	defer conn.Close()

	state := conn.ConnectionState()
	if len(state.PeerCertificates) == 0 {
		return fmt.Errorf("tls certificate not found")
	}
	cert := state.PeerCertificates[0]
	now := time.Now()
	if now.Before(cert.NotBefore) {
		return fmt.Errorf("tls certificate not valid before %s", cert.NotBefore.Format(time.RFC3339))
	}
	if now.After(cert.NotAfter) {
		return fmt.Errorf("tls certificate expired at %s", cert.NotAfter.Format(time.RFC3339))
	}
	return nil
}

func splitMonitorTarget(target string, defaultPort int) (string, int, error) {
	value := strings.TrimSpace(target)
	if value == "" {
		return "", 0, fmt.Errorf("target is empty")
	}

	if strings.Contains(value, "://") {
		parsed, err := url.Parse(value)
		if err != nil {
			return "", 0, err
		}
		host := parsed.Hostname()
		if host == "" {
			return "", 0, fmt.Errorf("target host is empty")
		}
		port := defaultPort
		if parsed.Port() != "" {
			if _, err := fmt.Sscanf(parsed.Port(), "%d", &port); err != nil {
				return "", 0, fmt.Errorf("invalid target port")
			}
		}
		return host, port, nil
	}

	host := value
	port := defaultPort
	if h, p, err := net.SplitHostPort(value); err == nil {
		host = h
		if _, err := fmt.Sscanf(p, "%d", &port); err != nil {
			return "", 0, fmt.Errorf("invalid target port")
		}
	}
	if host == "" {
		return "", 0, fmt.Errorf("target host is empty")
	}
	return host, port, nil
}
