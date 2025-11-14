package cli

import (
	"fmt"
	"os"
)

// ANSI颜色码
const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[31m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
	ColorBlue   = "\033[34m"
	ColorCyan   = "\033[36m"
	ColorWhite  = "\033[37m"
)

// 检查是否支持颜色输出（设置了TERM环境变量）
func supportsColor() bool {
	return os.Getenv("TERM") != ""
}

// printColor 打印彩色文本
func printColor(color, text string) {
	if supportsColor() {
		fmt.Printf("%s%s%s\n", color, text, ColorReset)
	} else {
		fmt.Println(text)
	}
}

// printSuccess 打印成功信息（绿色）
func printSuccess(text string) {
	printColor(ColorGreen, "✓ "+text)
}

// printError 打印错误信息（红色）
func printError(text string) {
	printColor(ColorRed, "✗ "+text)
}

// printWarning 打印警告信息（黄色）
func printWarning(text string) {
	printColor(ColorYellow, "⚠ "+text)
}

// printInfo 打印信息（蓝色）
func printInfo(text string) {
	printColor(ColorBlue, "ℹ "+text)
}

// printStatus 打印状态信息（带颜色）
func printStatus(status, text string) {
	var color string
	switch status {
	case "running", "active", "success":
		color = ColorGreen
	case "stopped", "inactive":
		color = ColorYellow
	case "failed", "error":
		color = ColorRed
	case "starting", "activating":
		color = ColorCyan
	case "stopping", "deactivating":
		color = ColorYellow
	default:
		color = ColorWhite
	}
	printColor(color, text)
}

