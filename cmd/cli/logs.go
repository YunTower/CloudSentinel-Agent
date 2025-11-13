package cli

import (
	"os"
	"os/exec"
	"strconv"

	"github.com/spf13/cobra"
)

var followFlag bool
var linesFlag int

// logsCmd 日志命令
var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "查看agent日志",
	Long:  `查看CloudSentinel Agent的日志。如果使用systemd管理，将显示journal日志。`,
	RunE:  runLogs,
}

func init() {
	logsCmd.Flags().BoolVarP(&followFlag, "follow", "f", false, "跟踪日志输出（类似tail -f）")
	logsCmd.Flags().IntVarP(&linesFlag, "lines", "n", 50, "显示最后N行日志")
	rootCmd.AddCommand(logsCmd)
}

func runLogs(cmd *cobra.Command, args []string) error {
	// 检查systemd服务是否存在
	servicePath := "/etc/systemd/system/cloudsentinel-agent.service"
	if _, err := os.Stat(servicePath); err == nil {
		// 使用journalctl查看日志
		journalArgs := []string{"-u", "cloudsentinel-agent.service"}
		if followFlag {
			journalArgs = append(journalArgs, "-f")
		} else {
			journalArgs = append(journalArgs, "-n", strconv.Itoa(linesFlag))
		}

		journalCmd := exec.Command("journalctl", journalArgs...)
		journalCmd.Stdout = os.Stdout
		journalCmd.Stderr = os.Stderr
		return journalCmd.Run()
	}

	// 如果没有systemd服务，提示用户查看日志文件
	cmd.Println("未找到systemd服务，请查看日志文件目录（默认: logs/）")
	return nil
}
