package websocket

import (
	"agent/internal/crypto"
	"agent/internal/logger"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Message struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

type Client struct {
	API           string
	Conn          *websocket.Conn
	Logger        *logger.Logger
	IsConnected   bool
	ReconnectWait time.Duration
	MaxReconnect  int
	mu            sync.Mutex
	stopChan      chan struct{}
	// 加密相关字段
	SessionKey        []byte // AES 会话密钥
	EncryptionEnabled bool   // 是否启用加密
}

func NewClient(api string, logger *logger.Logger) *Client {
	return &Client{
		API:           api,
		Logger:        logger,
		IsConnected:   false,
		ReconnectWait: 5 * time.Second,
		MaxReconnect:  5, // 最多重连5次
		stopChan:      make(chan struct{}),
	}
}

func (c *Client) Connect() error {
	conn, _, err := websocket.DefaultDialer.Dial(c.API, nil)
	if err != nil {
		return fmt.Errorf("连接失败: %v", err)
	}

	c.mu.Lock()
	c.Conn = conn
	c.IsConnected = true
	c.mu.Unlock()

	return nil
}

func (c *Client) ConnectWithRetry() error {
	attempts := 0
	for {
		select {
		case <-c.stopChan:
			return fmt.Errorf("连接已停止")
		default:
			err := c.Connect()
			if err == nil {
				c.Logger.Success("WebSocket 连接成功")
				return nil
			}

			attempts++
			if c.MaxReconnect > 0 && attempts >= c.MaxReconnect {
				return fmt.Errorf("达到最大重连次数(%d): %v", c.MaxReconnect, err)
			}

			// 格式化最大重连次数显示
			maxReconnectStr := "∞"
			if c.MaxReconnect > 0 {
				maxReconnectStr = fmt.Sprintf("%d", c.MaxReconnect)
			}

			c.Logger.Warn("连接失败(尝试 %d/%s): %v，%.0f秒后重试...",
				attempts,
				maxReconnectStr,
				err,
				c.ReconnectWait.Seconds())

			time.Sleep(c.ReconnectWait)
		}
	}
}

func (c *Client) Reconnect() error {
	c.mu.Lock()
	if c.Conn != nil {
		c.Conn.Close()
	}
	c.IsConnected = false
	c.mu.Unlock()

	c.Logger.Warn("开始重新连接...")
	return c.ConnectWithRetry()
}

// StartHeartbeat 启动心跳进程，使用 context 控制生命周期
func (c *Client) StartHeartbeat(ctx context.Context, healthChan chan<- bool, interval time.Duration) {
	if interval <= 0 {
		interval = 20 * time.Second // 默认20秒
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	c.Logger.Info("心跳进程：已启动")

	// 在开始前检查连接状态
	if !c.IsConnected || c.Conn == nil {
		c.Logger.Warn("心跳进程：WebSocket 未连接，等待连接...")
		// 等待连接或 context 取消
		select {
		case <-ctx.Done():
			c.Logger.Info("心跳进程：已停止")
			return
		case <-time.After(5 * time.Second):
			// 5秒后如果仍未连接，返回让进程管理器处理
			c.Logger.Warn("心跳进程：等待连接超时，退出")
			select {
			case healthChan <- false:
			default:
			}
			return
		}
	}

	for {
		select {
		case <-ticker.C:
			// 检查连接状态
			if !c.IsConnected || c.Conn == nil {
				c.Logger.Warn("心跳进程：连接已断开，等待重连...")
				// 上报不健康状态
				select {
				case healthChan <- false:
				default:
				}
				// 等待重连，最多等待30秒
				reconnectTimeout := time.After(30 * time.Second)
				checkTicker := time.NewTicker(5 * time.Second)

				for {
					select {
					case <-ctx.Done():
						checkTicker.Stop()
						c.Logger.Info("心跳进程：已停止")
						return
					case <-reconnectTimeout:
						// 30秒后如果仍未连接，返回让进程管理器处理
						checkTicker.Stop()
						c.Logger.Warn("心跳进程：等待重连超时，退出")
						return
					case <-checkTicker.C:
						// 每5秒检查一次连接状态
						if c.IsConnected && c.Conn != nil {
							checkTicker.Stop()
							c.Logger.Info("心跳进程：连接已恢复，继续心跳")
							goto continueHeartbeat
						}
					}
				}
			continueHeartbeat:
				continue
			}

			heartbeatMessage := Message{
				Type: "hello",
			}
			if err := c.SendMessage(heartbeatMessage); err != nil {
				c.Logger.Error("心跳发送失败: %v", err)
				// 上报不健康状态
				select {
				case healthChan <- false:
				default:
				}
				// 发送失败时，不立即返回，继续等待下次 ticker
				// 如果连接断开，会在下次检查时处理
				continue
			}
			// 上报健康状态
			select {
			case healthChan <- true:
			default:
			}
		case <-ctx.Done():
			c.Logger.Info("心跳进程：已停止")
			return
		}
	}
}

func (c *Client) SendMessage(content interface{}) error {
	// 如果启用了加密，使用加密发送
	if c.IsEncryptionEnabled() {
		return c.WriteEncryptedJSON(content)
	}
	return c.writePlainJSON(content)
}

// writePlainJSON 发送明文 JSON 消息
func (c *Client) writePlainJSON(content interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.IsConnected || c.Conn == nil {
		return fmt.Errorf("未连接")
	}

	data, err := json.Marshal(content)
	if err != nil {
		c.Logger.Error("将内容转换为 JSON 时出错: %v", err)
		return err
	}

	err = c.Conn.WriteMessage(websocket.TextMessage, data)
	if err != nil {
		c.Logger.Error("发送消息时出错: %v", err)
		c.IsConnected = false
		return err
	}

	return nil
}

// WriteEncryptedJSON 发送加密的 JSON 消息
func (c *Client) WriteEncryptedJSON(v interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.IsConnected || c.Conn == nil {
		return fmt.Errorf("未连接")
	}

	// 获取会话密钥
	sessionKey := c.getSessionKey()
	if sessionKey == nil {
		return fmt.Errorf("会话密钥未设置")
	}

	// 序列化 JSON
	jsonData, err := json.Marshal(v)
	if err != nil {
		return err
	}

	// 使用 AES 加密
	encryptedData, err := crypto.EncryptMessage(jsonData, sessionKey)
	if err != nil {
		return err
	}

	// 直接发送二进制消息
	err = c.Conn.WriteMessage(websocket.BinaryMessage, encryptedData)
	if err != nil {
		c.Logger.Error("发送加密消息时出错: %v", err)
		c.IsConnected = false
		return err
	}

	return nil
}

// ReadEncryptedMessage 读取加密消息
func (c *Client) ReadEncryptedMessage() ([]byte, error) {
	if !c.IsEncryptionEnabled() {
		// 未启用加密，使用普通方式读取
		_, message, err := c.Conn.ReadMessage()
		return message, err
	}

	// 获取会话密钥
	sessionKey := c.getSessionKey()
	if sessionKey == nil {
		return nil, fmt.Errorf("会话密钥未设置")
	}

	// 读取消息
	messageType, message, err := c.Conn.ReadMessage()
	if err != nil {
		return nil, err
	}

	// 如果是二进制消息，直接解密
	if messageType == websocket.BinaryMessage {
		decryptedData, err := crypto.DecryptMessage(message, sessionKey)
		if err != nil {
			return nil, err
		}
		return decryptedData, nil
	}

	// 尝试解析为 JSON
	var msg map[string]interface{}
	if err := json.Unmarshal(message, &msg); err == nil {
		// 检查是否是加密消息
		if encrypted, ok := msg["encrypted"].(bool); ok && encrypted {
			// JSON 包装的加密消息
			encryptedDataBase64, ok := msg["data"].(string)
			if !ok {
				return nil, fmt.Errorf("无效的加密消息格式")
			}
			encryptedData, err := base64.StdEncoding.DecodeString(encryptedDataBase64)
			if err != nil {
				return nil, err
			}
			decryptedData, err := crypto.DecryptMessage(encryptedData, sessionKey)
			if err != nil {
				return nil, err
			}
			return decryptedData, nil
		}
	}

	// 不是加密消息，直接返回原始数据
	return message, nil
}

// EnableEncryption 启用加密
func (c *Client) EnableEncryption(sessionKey []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.SessionKey = make([]byte, len(sessionKey))
	copy(c.SessionKey, sessionKey)
	c.EncryptionEnabled = true
}

// IsEncryptionEnabled 检查是否启用加密
func (c *Client) IsEncryptionEnabled() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.EncryptionEnabled
}

// getSessionKey 获取会话密钥（内部方法，需要加锁）
func (c *Client) getSessionKey() []byte {
	if c.SessionKey == nil {
		return nil
	}
	key := make([]byte, len(c.SessionKey))
	copy(key, c.SessionKey)
	return key
}

func (c *Client) Close() {
	c.mu.Lock()
	select {
	case <-c.stopChan:
		// 已经关闭
		c.mu.Unlock()
		return
	default:
		close(c.stopChan)
	}
	c.mu.Unlock()

	c.mu.Lock()
	if c.Conn != nil {
		c.Conn.Close()
	}
	c.IsConnected = false
	c.mu.Unlock()
	c.Logger.Info("WebSocket 连接已关闭")
}

// IsStopped 检查客户端是否已停止
func (c *Client) IsStopped() bool {
	select {
	case <-c.stopChan:
		return true
	default:
		return false
	}
}

func (c *Client) GetConnection() *websocket.Conn {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.Conn
}

// 向后兼容的函数
func Connect(api string) (*websocket.Conn, error) {
	conn, _, err := websocket.DefaultDialer.Dial(api, nil)
	if err != nil {
		return nil, fmt.Errorf("连接失败: %v", err)
	}
	return conn, nil
}

func SendMessage(content interface{}, conn *websocket.Conn, logger *logger.Logger) {
	data, err := json.Marshal(content)
	if err != nil {
		logger.Error("将内容转换为 JSON 时出错: %v", err)
		return
	}
	err = conn.WriteMessage(websocket.TextMessage, data)
	if err != nil {
		logger.Error("发送消息时出错: %v", err)
		return
	}
}
