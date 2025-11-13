//go:build !linux
// +build !linux

package daemon

import (
	"fmt"
)

// Daemonize 将当前进程转换为守护进程（仅Linux支持）
func Daemonize() error {
	return fmt.Errorf("守护进程模式仅在Linux上支持")
}
