package cli

import (
	"github.com/spf13/cobra"
)

// versionCmd 版本命令
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "显示版本信息",
	Long:  `显示CloudSentinel Agent的版本信息。`,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Println(rootCmd.Version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
