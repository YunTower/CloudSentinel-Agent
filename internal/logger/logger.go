package logger

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	Reset  = "\033[0m"
	Red    = "\033[31m"
	Green  = "\033[32m"
	Yellow = "\033[33m"
	White  = "\033[37m"
)

type Logger struct {
	fileLogger    *log.Logger
	console       *log.Logger
	mu            sync.Mutex
	logDir        string
	file          *os.File
	currentDate   string
	retentionDays int
}

func NewLogger(logDir string, retentionDays int) (*Logger, error) {
	if err := os.MkdirAll(logDir, os.ModePerm); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	date := time.Now().Format("2006-01-02")
	filePath := filepath.Join(logDir, date+".txt")

	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	l := &Logger{
		fileLogger:    log.New(file, "", log.LstdFlags),
		console:       log.New(os.Stdout, "", log.LstdFlags),
		logDir:        logDir,
		file:          file,
		currentDate:   date,
		retentionDays: retentionDays,
	}

	// 启动后台任务：清理旧日志
	go l.startCleaner()

	return l, nil
}

// rotate 检查并轮转日志文件
func (l *Logger) rotate() error {
	today := time.Now().Format("2006-01-02")
	if today == l.currentDate {
		return nil
	}

	// 关闭旧文件
	if l.file != nil {
		l.file.Close()
	}

	// 打开新文件
	filePath := filepath.Join(l.logDir, today+".txt")
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		// 如果打开新文件失败，尝试重新打开 stdout 作为后备，避免 panic
		l.fileLogger.SetOutput(os.Stdout)
		return fmt.Errorf("failed to rotate log file: %w", err)
	}

	l.file = file
	l.fileLogger.SetOutput(file)
	l.currentDate = today

	// 每天轮转时也触发一次清理
	go l.clean()

	return nil
}

// startCleaner 启动日志清理任务
func (l *Logger) startCleaner() {
	// 立即执行一次清理
	l.clean()

	// 每天检查一次
	ticker := time.NewTicker(24 * time.Hour)
	go func() {
		for range ticker.C {
			l.clean()
		}
	}()
}

// clean 清理过期日志
func (l *Logger) clean() {
	if l.retentionDays <= 0 {
		return
	}

	entries, err := os.ReadDir(l.logDir)
	if err != nil {
		// 由于此时可能在后台运行，无法保证能写入日志，但尝试一下
		// 注意：不要死锁，clean 不加锁，但 Error 方法加锁
		l.Error("Failed to read log directory for cleaning: %v", err)
		return
	}

	cutoff := time.Now().AddDate(0, 0, -l.retentionDays)
	// 将 cutoff 的时间部分设为 00:00:00，确保只比较日期
	cutoff = time.Date(cutoff.Year(), cutoff.Month(), cutoff.Day(), 23, 59, 59, 0, cutoff.Location())

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		// 简单的文件名检查：假设是 2006-01-02.txt
		if len(name) < 10 {
			continue
		}

		fileDateStr := name[:10]
		fileDate, err := time.Parse("2006-01-02", fileDateStr)
		if err != nil {
			// 文件名不是日期格式，尝试使用修改时间
			info, err := entry.Info()
			if err == nil {
				if info.ModTime().Before(cutoff) {
					l.deleteLogFile(name)
				}
			}
			continue
		}

		// 如果文件日期早于截止日期（即文件日期 < cutoff），则删除
		if fileDate.Before(cutoff) {
			l.deleteLogFile(name)
		}
	}
}

func (l *Logger) deleteLogFile(name string) {
	path := filepath.Join(l.logDir, name)
	if err := os.Remove(path); err != nil {
		l.Error("Failed to remove old log file %s: %v", name, err)
	} else {
		l.Info("Removed old log file: %s", name)
	}
}

func (l *Logger) Info(format string, v ...interface{}) {
	l.log(White, "INFO", format, v...)
}

func (l *Logger) Warn(format string, v ...interface{}) {
	l.log(Yellow, "WARN", format, v...)
}

func (l *Logger) Error(format string, v ...interface{}) {
	l.log(Red, "ERROR", format, v...)
}

func (l *Logger) Success(format string, v ...interface{}) {
	l.log(Green, "SUCCESS", format, v...)
}

func (l *Logger) log(color, level, format string, v ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// 检查是否需要轮转
	if err := l.rotate(); err != nil {
		fmt.Fprintf(os.Stderr, "Log rotation failed: %v\n", err)
	}

	msg := fmt.Sprintf(format, v...)
	l.fileLogger.Printf("[%s] %s", level, msg)
	l.console.Printf("%s[%s] %s%s", color, level, msg, Reset)
}
