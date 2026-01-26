package cli

import (
	"agent/internal/version"
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

// versionCmd 版本命令
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "显示版本信息",
	Long:  `显示CloudSentinel Agent的版本及构建信息。`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("CloudSentinel Agent v%s\n", version.AgentVersion)
		fmt.Printf("Git Commit: %s\n", version.GitCommit)
		fmt.Printf("Build Time: %s\n", version.BuildTime)
		fmt.Printf("Go Version: %s\n", runtime.Version())
		fmt.Printf("OS/Arch:    %s/%s\n", runtime.GOOS, runtime.GOARCH)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
