package websocket

import (
	"agent/internal/logger"
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
	heartbeatStop chan struct{}
}

func NewClient(api string, logger *logger.Logger) *Client {
	return &Client{
		API:           api,
		Logger:        logger,
		IsConnected:   false,
		ReconnectWait: 5 * time.Second,
		MaxReconnect:  0, // 0表示无限重连
		stopChan:      make(chan struct{}),
		heartbeatStop: make(chan struct{}),
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

func (c *Client) StartHeartbeat() {
	c.heartbeatStop = make(chan struct{})
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			heartbeatMessage := Message{
				Type: "hello",
			}
			if err := c.SendMessage(heartbeatMessage); err != nil {
				c.Logger.Error("心跳发送失败: %v", err)
			}
		case <-c.heartbeatStop:
			c.Logger.Info("心跳停止")
			return
		}
	}
}

func (c *Client) StopHeartbeat() {
	close(c.heartbeatStop)
}

func (c *Client) SendMessage(content interface{}) error {
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

func (c *Client) Close() {
	close(c.stopChan)
	c.StopHeartbeat()
	
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if c.Conn != nil {
		c.Conn.Close()
	}
	c.IsConnected = false
	c.Logger.Info("WebSocket 连接已关闭")
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
