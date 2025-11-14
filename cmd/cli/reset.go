package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"agent/config"

	"github.com/spf13/cobra"
)

var yesFlag bool

// resetCmd 重置命令
var resetCmd = &cobra.Command{
	Use:   "reset",
	Short: "重置配置文件",
	Long:  `删除配置文件并进入交互式配置模式。`,
	RunE:  runReset,
}

func init() {
	resetCmd.Flags().BoolVarP(&yesFlag, "yes", "y", false, "跳过确认提示")
	rootCmd.AddCommand(resetCmd)
}

func runReset(cmd *cobra.Command, args []string) error {
	// 获取配置文件路径
	cfgPath := configPath
	if cfgPath == "" {
		cfgPath = config.GetConfigPath()
	}

	// 确认删除
	if !yesFlag {
		fmt.Printf("确定要删除配置文件 %s 吗？(y/N): ", cfgPath)
		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer != "y" && answer != "yes" {
			printInfo("已取消")
			return nil
		}
	}

	// 删除配置文件
	if err := os.Remove(cfgPath); err != nil && !os.IsNotExist(err) {
		printError(fmt.Sprintf("删除配置文件失败: %v", err))
		return err
	}

	printSuccess(fmt.Sprintf("配置文件 %s 已删除", cfgPath))
	printInfo("请运行 './agent start' 进入交互式配置模式")

	return nil
}
