package reporter

import (
	"agent/config"
	"agent/internal/collector"
	"agent/internal/logger"
	"agent/internal/system"
	"agent/internal/websocket"
	"encoding/json"
	"fmt"
	"io"
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
	
	// 调试：记录发送的认证信息（不记录完整的key，只记录前4个字符）
	keyPreview := cfg.Key
	if len(keyPreview) > 8 {
		keyPreview = keyPreview[:8] + "..."
	}
	logger.Info("准备发送认证消息，key: %s", keyPreview)
	
	if err := client.SendMessage(authMessage); err != nil {
		logger.Error("发送认证消息失败: %v", err)
	} else {
		logger.Info("已发送认证消息")
	}
}

func StartReporter(client *websocket.Client, system *system.System, logger *logger.Logger, cfg config.Config) {
	col := collector.NewCollector(system, logger, client)
	isDataReportingStarted := false
	
	// 连接成功后立即发送认证消息
	sendAuthMessage(client, cfg, logger)
	
	for {
		conn := client.GetConnection()
		if conn == nil {
			logger.Error("连接不可用，尝试重连...")
			if err := client.Reconnect(); err != nil {
				logger.Error("重连失败: %v", err)
				time.Sleep(5 * time.Second)
				continue
			}
			isDataReportingStarted = false
			conn = client.GetConnection()
			// 重连成功后立即发送认证消息
			sendAuthMessage(client, cfg, logger)
		}
		
		_, message, err := conn.ReadMessage()
		if err != nil {
			if err == io.EOF {
				logger.Warn("连接已关闭")
			} else {
				logger.Error(fmt.Sprintf("读取消息时出错: %v", err))
			}
			
			client.IsConnected = false
			logger.Warn("连接断开，尝试重连...")
			
			if err := client.Reconnect(); err != nil {
				logger.Error("重连失败: %v", err)
				time.Sleep(5 * time.Second)
			} else {
				isDataReportingStarted = false
				// 重连成功后立即发送认证消息
				sendAuthMessage(client, cfg, logger)
			}
			continue
		}

		logger.Info(fmt.Sprintf("收到消息: %s", message))
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
			
			// 认证成功后启动数据收集（仅启动一次）
			if !isDataReportingStarted {
				isDataReportingStarted = true
				go col.StartPeriodicReporting()
			}
		}

		// 处理带状态和消息的响应
		if statusExists && messageExists {
			if statusValue != "success" {
				logger.Warn("%s: %s", typeValue, messageValue)
			} else {
				logger.Success("%s: %s", typeValue, messageValue)
			}
		} else {
			// 处理服务器请求
			if !statusExists {
				switch typeValue {
				case "hello":
					// 服务器发送hello（心跳响应）
					// logger.Info("收到心跳响应")
				case "auth":
					// 服务器要求认证（虽然agent会主动认证，但保留此逻辑作为备用）
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
					} else {
						logger.Info("已发送认证消息（响应服务器请求）")
					}
				default:
					logger.Warn("未知的消息类型: %v", typeValue)
				}
			}
		}
	}
}

//func reportSystemStatus(conn *websocket2.Conn, system *system.System, logger *logger.Logger) {
//	cpuStatus, _ := marshalJSON(system.GetCpuUsedPercent(), logger)
//	_memoryStatus := map[string]int{
//		"total":        system.GetMemoryTotal(),
//		"used":         system.GetMemoryUsed(),
//		"used_percent": system.GetMemoryUsedPercent(),
//		"free":         system.GetMemoryFree(),
//	}
//	memoryStatus, _ := marshalJSON(_memoryStatus, logger)
//
//	var _diskData []DiskIo
//	for _, item := range system.GetDiskAllPart() {
//		info := system.GetDiskIO(item.Device)
//		_diskData = append(_diskData, DiskIo{
//			Path:        item.Device,
//			Total:       int(info.Total),
//			Free:        int(info.Free),
//			Used:        int(info.Used),
//			UsedPercent: info.UsedPercent,
//		})
//	}
//	diskData, _ := marshalJSON(_diskData, logger)
//	systemStatus := map[string]interface{}{
//		"cpu":    json.RawMessage(cpuStatus),
//		"memory": json.RawMessage(memoryStatus),
//		"disk":   json.RawMessage(diskData),
//		"os":     runtime.GOOS,
//		"arch":   runtime.GOARCH,
//	}
//
//	content := websocket.Message{
//		Type: "status",
//		Data: systemStatus,
//	}
//	websocket.SendMessage(content, conn, logger)
//}

func marshalJSON(data interface{}, logger *logger.Logger) ([]byte, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {

		logger.Error("序列化数据时出错: %v", err)
	}
	return jsonData, err
}
