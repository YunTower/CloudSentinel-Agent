package reporter

import (
	"agent/config"
	"agent/internal/logger"
	"agent/internal/websocket"
	"encoding/json"
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
func sendAuthMessage(client *websocket.Client, cfg config.Config, logger *logger.Logger) {
	// 验证key是否存在
	if cfg.Key == "" {
		logger.Error("agent key为空，无法发送认证消息")
		return
	}

	authData := map[string]interface{}{
		"type": "server",
		"key":  cfg.Key,
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
	// 连接成功后立即发送认证消息
	sendAuthMessage(client, cfg, logger)

	// 消息读取循环
	for {
		conn := client.GetConnection()
		if conn == nil {
			logger.Error("连接不可用，尝试重连...")
			if err := client.Reconnect(); err != nil {
				logger.Error("重连失败: %v", err)
				time.Sleep(5 * time.Second)
				continue
			}
			conn = client.GetConnection()
			// 重连成功后立即发送认证消息
			sendAuthMessage(client, cfg, logger)
			// 通知断开连接，让主进程重启子进程
			if callbacks.OnDisconnect != nil {
				callbacks.OnDisconnect()
			}
		}

		// 设置读取超时，防止阻塞
		conn.SetReadDeadline(time.Now().Add(90 * time.Second))

		_, message, err := conn.ReadMessage()
		if err != nil {
			if err == io.EOF {
				logger.Warn("连接已关闭")
			} else {
				logger.Error("读取消息时出错: %v", err)
			}

			client.IsConnected = false

			if err := client.Reconnect(); err != nil {
				logger.Error("重连失败: %v", err)
				logger.Error("已达最大重连次数，请检查网络连接或后端服务状态")
				time.Sleep(60 * time.Second)
				// 重置连接状态，允许下一轮重连
				continue
			} else {
				// 重连成功后立即发送认证消息
				sendAuthMessage(client, cfg, logger)
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
						} else {
							logger.Warn("未知的命令: %s", commandData)
						}
					}
				case "auth":
					// 服务器要求认证
					authData := map[string]interface{}{
						"type": "server",
						"key":  cfg.Key,
					}
					authMessage := websocket.Message{
						Type: "auth",
						Data: authData,
					}
					if err := client.SendMessage(authMessage); err != nil {
						logger.Error("发送认证消息失败: %v", err)
					}
				default:
					logger.Warn("未知的消息类型: %v", typeValue)
				}
			}
		}
	}
}
