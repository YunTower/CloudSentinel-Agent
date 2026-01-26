package cli

import (
	"agent/config"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"

	"github.com/spf13/cobra"
)

// logsCmd 日志命令
var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "查看日志",
	Long:  `查看CloudSentinel Agent的最新日志文件。`,
	RunE:  runLogs,
}

var (
	linesFlag  int
	followFlag bool
)

func init() {
	logsCmd.Flags().BoolVarP(&followFlag, "follow", "f", false, "跟踪日志输出")
	logsCmd.Flags().IntVarP(&linesFlag, "lines", "n", 50, "显示最后N行")
	rootCmd.AddCommand(logsCmd)
}

func runLogs(cmd *cobra.Command, args []string) error {
	cfgPath := configPath
	if cfgPath == "" {
		cfgPath = config.GetConfigPath()
	}

	// 尝试加载配置以获取日志路径
	var logDir string
	cfg, err := config.LoadConfigFromFile(cfgPath)
	if err != nil {
		logDir = "logs"
	} else {
		logDir = cfg.LogPath
	}

	// 确保绝对路径
	if !filepath.IsAbs(logDir) {
		execPath, _ := os.Executable()
		execDir := filepath.Dir(execPath)
		logDir = filepath.Join(execDir, logDir)
	}

	// 检查目录是否存在
	if _, err := os.Stat(logDir); os.IsNotExist(err) {
		return fmt.Errorf("日志目录不存在: %s", logDir)
	}

	// 查找最新的日志文件
	entries, err := os.ReadDir(logDir)
	if err != nil {
		return fmt.Errorf("无法读取日志目录: %w", err)
	}

	var logFiles []os.DirEntry
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".log" {
			logFiles = append(logFiles, e)
		}
	}

	if len(logFiles) == 0 {
		return fmt.Errorf("在 %s 中未找到日志文件", logDir)
	}

	// 按修改时间倒序排序
	sort.Slice(logFiles, func(i, j int) bool {
		infoI, _ := logFiles[i].Info()
		infoJ, _ := logFiles[j].Info()
		return infoI.ModTime().After(infoJ.ModTime())
	})

	latestLog := filepath.Join(logDir, logFiles[0].Name())
	printInfo(fmt.Sprintf("查看日志文件: %s", latestLog))

	// 执行查看命令
	if runtime.GOOS == "windows" {
		psArgs := []string{"Get-Content", "-Path", latestLog, "-Tail", strconv.Itoa(linesFlag)}
		if followFlag {
			psArgs = append(psArgs, "-Wait")
		}
		c := exec.Command("powershell", psArgs...)
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		c.Stdin = os.Stdin
		return c.Run()
	} else {
		tailArgs := []string{"-n", strconv.Itoa(linesFlag)}
		if followFlag {
			tailArgs = append(tailArgs, "-f")
		}
		tailArgs = append(tailArgs, latestLog)
		c := exec.Command("tail", tailArgs...)
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		c.Stdin = os.Stdin
		return c.Run()
	}
}
