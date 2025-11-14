package systemd

import (
	"fmt"
	"os"
	"path/filepath"
)

const serviceTemplate = `[Unit]
Description=CloudSentinel Agent
After=network.target

[Service]
Type=simple
ExecStart=%s start
Restart=always
RestartSec=5
User=root
WorkingDirectory=%s
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
`

// GenerateServiceFile 生成systemd service文件内容
func GenerateServiceFile(execPath string) string {
	// 获取可执行文件所在目录作为工作目录
	workingDir := filepath.Dir(execPath)
	return fmt.Sprintf(serviceTemplate, execPath, workingDir)
}

// GetServiceFilePath 获取service文件路径
func GetServiceFilePath() string {
	return "/etc/systemd/system/cloudsentinel-agent.service"
}

// InstallService 安装systemd服务
func InstallService(execPath string) error {
	// 获取绝对路径
	absPath, err := filepath.Abs(execPath)
	if err != nil {
		return fmt.Errorf("获取绝对路径失败: %w", err)
	}

	// 检查可执行文件是否存在
	if _, err := os.Stat(absPath); err != nil {
		return fmt.Errorf("可执行文件不存在: %s", absPath)
	}

	// 生成service文件内容
	serviceContent := GenerateServiceFile(absPath)

	// 写入service文件
	servicePath := GetServiceFilePath()

	// 确保目录存在
	dir := filepath.Dir(servicePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建systemd目录失败: %w", err)
	}

	if err := os.WriteFile(servicePath, []byte(serviceContent), 0644); err != nil {
		return fmt.Errorf("写入service文件失败: %w", err)
	}

	return nil
}

// UninstallService 卸载systemd服务
func UninstallService() error {
	servicePath := GetServiceFilePath()
	if err := os.Remove(servicePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("删除service文件失败: %w", err)
	}
	return nil
}

// ServiceExists 检查service文件是否存在
func ServiceExists() bool {
	servicePath := GetServiceFilePath()
	_, err := os.Stat(servicePath)
	return err == nil
}
