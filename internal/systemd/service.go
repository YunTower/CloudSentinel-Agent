package systemd

import (
	"fmt"
	"os/exec"
	"strings"
)

// ReloadDaemon 重新加载systemd daemon
func ReloadDaemon() error {
	cmd := exec.Command("systemctl", "daemon-reload")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("重新加载systemd daemon失败: %w", err)
	}
	return nil
}

// EnableService 启用systemd服务（开机自启）
func EnableService() error {
	cmd := exec.Command("systemctl", "enable", "cloudsentinel-agent.service")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("启用服务失败: %w", err)
	}
	return nil
}

// DisableService 禁用systemd服务
func DisableService() error {
	cmd := exec.Command("systemctl", "disable", "cloudsentinel-agent.service")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("禁用服务失败: %w", err)
	}
	return nil
}

// StartService 启动systemd服务
func StartService() error {
	cmd := exec.Command("systemctl", "start", "cloudsentinel-agent.service")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("启动服务失败: %w, 输出: %s", err, string(output))
	}
	return nil
}

// StopService 停止systemd服务
func StopService() error {
	cmd := exec.Command("systemctl", "stop", "cloudsentinel-agent.service")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("停止服务失败: %w", err)
	}
	return nil
}

// RestartService 重启systemd服务
func RestartService() error {
	cmd := exec.Command("systemctl", "restart", "cloudsentinel-agent.service")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("重启服务失败: %w", err)
	}
	return nil
}

// GetServiceStatus 获取服务状态
func GetServiceStatus() (string, error) {
	cmd := exec.Command("systemctl", "is-active", "cloudsentinel-agent.service")
	output, err := cmd.Output()
	if err != nil {
		return "inactive", nil
	}
	return strings.TrimSpace(string(output)), nil
}

// IsServiceActive 检查服务是否处于活动状态
func IsServiceActive() (bool, error) {
	status, err := GetServiceStatus()
	if err != nil {
		return false, err
	}
	return status == "active", nil
}
