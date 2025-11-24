package reporter

import (
	"agent/internal/logger"
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// UpdateService Agent 更新服务
type UpdateService struct {
	logger *logger.Logger
}

// NewUpdateService 创建更新服务实例
func NewUpdateService(logger *logger.Logger) *UpdateService {
	return &UpdateService{
		logger: logger,
	}
}

// UpdateAgent 执行 Agent 更新
func (s *UpdateService) UpdateAgent(version, versionType string) error {
	s.logger.Info("开始更新 Agent，目标版本: %s-%s", version, versionType)

	// 从 GitHub 获取最新 release 信息
	releaseUrl := "https://api.github.com/repos/YunTower/CloudSentinel-Agent/releases/latest"
	resp, err := http.Get(releaseUrl)
	if err != nil {
		return fmt.Errorf("获取 release 信息失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("获取 release 信息失败，状态码: %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("解析 release 信息失败: %v", err)
	}

	// 获取系统信息
	osType, arch := s.getSystemInfo()
	s.logger.Info("检测到系统: %s-%s", osType, arch)

	// 查找匹配的二进制包
	assets, ok := result["assets"].([]interface{})
	if !ok {
		return fmt.Errorf("未找到发布文件列表")
	}

	fileName, downloadUrl := s.findAssetByArchitecture(assets, osType, arch)
	if fileName == "" {
		return fmt.Errorf("未找到适用于 %s-%s 的软件包", osType, arch)
	}

	s.logger.Info("找到软件包: %s", fileName)

	// 下载二进制包
	downloadPath := filepath.Base(fileName)
	if err := s.downloadFile(downloadUrl, downloadPath, nil); err != nil {
		return fmt.Errorf("下载失败: %v", err)
	}

	s.logger.Info("软件包下载完成")

	// 查找并下载 SHA256 文件
	sha256FileName, sha256DownloadUrl := s.findSHA256Asset(assets, osType, arch)
	if sha256FileName == "" {
		return fmt.Errorf("未找到 SHA256 校验文件")
	}

	sha256Path := filepath.Base(sha256FileName)
	if err := s.downloadFile(sha256DownloadUrl, sha256Path, nil); err != nil {
		return fmt.Errorf("下载 SHA256 文件失败: %v", err)
	}

	s.logger.Info("校验文件下载完成")

	// 校验文件完整性
	expectedSHA256, err := s.readSHA256File(sha256Path)
	if err != nil {
		return fmt.Errorf("读取 SHA256 文件失败: %v", err)
	}

	actualSHA256, err := s.calculateSHA256(downloadPath)
	if err != nil {
		return fmt.Errorf("计算文件 SHA256 失败: %v", err)
	}

	if !strings.EqualFold(expectedSHA256, actualSHA256) {
		return fmt.Errorf("文件校验失败: 期望 %s, 实际 %s", expectedSHA256, actualSHA256)
	}

	s.logger.Info("文件校验通过")

	// 解压 tar.gz 文件
	extractDir := "update_extract"
	if err := os.RemoveAll(extractDir); err != nil {
		return fmt.Errorf("清理解压目录失败: %v", err)
	}
	if err := os.MkdirAll(extractDir, 0755); err != nil {
		return fmt.Errorf("创建解压目录失败: %v", err)
	}

	if err := s.extractTarGz(downloadPath, extractDir); err != nil {
		return fmt.Errorf("解压失败: %v", err)
	}

	s.logger.Info("解压完成")

	// 查找解压后的二进制文件
	binaryName := fmt.Sprintf("agent-%s-%s", osType, arch)
	if osType == "windows" {
		binaryName = fmt.Sprintf("agent-%s-%s.exe", osType, arch)
	}
	extractedBinaryPath := filepath.Join(extractDir, binaryName)

	// 检查文件是否存在
	if _, err := os.Stat(extractedBinaryPath); os.IsNotExist(err) {
		// 尝试在子目录中查找
		files, err := os.ReadDir(extractDir)
		if err != nil {
			return fmt.Errorf("读取解压目录失败: %v", err)
		}

		found := false
		for _, file := range files {
			if file.IsDir() {
				subPath := filepath.Join(extractDir, file.Name(), binaryName)
				if _, err := os.Stat(subPath); err == nil {
					extractedBinaryPath = subPath
					found = true
					break
				}
			}
		}

		if !found {
			return fmt.Errorf("解压后未找到二进制文件")
		}
	}

	// 备份当前文件
	currentExecPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("获取当前可执行文件路径失败: %v", err)
	}

	backupPath := currentExecPath + ".backup"
	if err := s.copyFile(currentExecPath, backupPath); err != nil {
		return fmt.Errorf("备份当前文件失败: %v", err)
	}

	// 替换文件
	if err := s.copyFile(extractedBinaryPath, currentExecPath); err != nil {
		// 恢复备份
		if restoreErr := s.copyFile(backupPath, currentExecPath); restoreErr != nil {
			return fmt.Errorf("替换文件失败且恢复备份也失败: %v, %v", err, restoreErr)
		}
		return fmt.Errorf("替换文件失败: %v", err)
	}

	// 设置可执行权限
	if err := os.Chmod(currentExecPath, 0755); err != nil {
		s.logger.Warn("设置可执行权限失败: %v", err)
	}

	s.logger.Info("文件替换完成")

	// 清理临时文件
	s.cleanupTempFiles(downloadPath, sha256Path, extractDir)

	// 重启应用
	time.Sleep(1 * time.Second)
	if err := s.restartApplication(); err != nil {
		return fmt.Errorf("重启应用失败: %v", err)
	}

	return nil
}

// getSystemInfo 获取系统信息
func (s *UpdateService) getSystemInfo() (osType, arch string) {
	osType = runtime.GOOS
	arch = runtime.GOARCH

	// 标准化 OS 名称
	if osType == "darwin" {
		osType = "darwin"
	} else if osType == "linux" {
		osType = "linux"
	} else if osType == "windows" {
		osType = "windows"
	}

	return osType, arch
}

// findAssetByArchitecture 查找匹配的二进制包
func (s *UpdateService) findAssetByArchitecture(assets []interface{}, osType, arch string) (string, string) {
	expectedName := fmt.Sprintf("agent-%s-%s.tar.gz", osType, arch)

	for _, asset := range assets {
		assetMap, ok := asset.(map[string]interface{})
		if !ok {
			continue
		}

		name, ok := assetMap["name"].(string)
		if !ok {
			continue
		}

		if name == expectedName {
			if url, ok := assetMap["browser_download_url"].(string); ok && url != "" {
				return name, url
			}
		}
	}

	return "", ""
}

// findSHA256Asset 查找 SHA256 文件
func (s *UpdateService) findSHA256Asset(assets []interface{}, osType, arch string) (string, string) {
	expectedName := fmt.Sprintf("agent-%s-%s.sha256", osType, arch)

	for _, asset := range assets {
		assetMap, ok := asset.(map[string]interface{})
		if !ok {
			continue
		}

		name, ok := assetMap["name"].(string)
		if !ok {
			continue
		}

		if name == expectedName {
			if url, ok := assetMap["browser_download_url"].(string); ok && url != "" {
				return name, url
			}
		}
	}

	return "", ""
}

// downloadFile 下载文件
func (s *UpdateService) downloadFile(url, filePath string, progressCallback func(int)) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("下载请求失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("下载失败，状态码: %d", resp.StatusCode)
	}

	// 创建目录
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建目录失败: %v", err)
	}

	// 创建文件
	out, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("创建文件失败: %v", err)
	}
	defer out.Close()

	// 复制数据
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf("写入文件失败: %v", err)
	}

	return nil
}

// calculateSHA256 计算文件 SHA256
func (s *UpdateService) calculateSHA256(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// readSHA256File 读取 SHA256 文件
func (s *UpdateService) readSHA256File(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}

	sha256 := strings.TrimSpace(string(data))
	return sha256, nil
}

// extractTarGz 解压 tar.gz 文件
func (s *UpdateService) extractTarGz(tarGzPath, destDir string) error {
	file, err := os.Open(tarGzPath)
	if err != nil {
		return err
	}
	defer file.Close()

	gzr, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		target := filepath.Join(destDir, header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			outFile, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(outFile, tr); err != nil {
				outFile.Close()
				return err
			}
			outFile.Close()
		}
	}

	return nil
}

// copyFile 复制文件
func (s *UpdateService) copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		return err
	}

	return destFile.Sync()
}

// cleanupTempFiles 清理临时文件
func (s *UpdateService) cleanupTempFiles(files ...string) {
	for _, file := range files {
		if err := os.RemoveAll(file); err != nil {
			s.logger.Warn("清理临时文件失败: %s, %v", file, err)
		}
	}
}

// restartApplication 重启应用程序
func (s *UpdateService) restartApplication() error {
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("获取可执行文件路径失败: %v", err)
	}

	pid := os.Getpid()

	// 构建重启命令
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command(execPath)
	} else {
		cmd = exec.Command(execPath)
	}

	cmd.Dir = filepath.Dir(execPath)
	cmd.Env = os.Environ()

	// 启动新进程
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动新进程失败: %v", err)
	}

	s.logger.Info("新进程已启动，PID: %d，正在终止当前进程 PID: %d", cmd.Process.Pid, pid)

	time.Sleep(500 * time.Millisecond)

	// 终止当前进程
	if runtime.GOOS == "windows" {
		killCmd := exec.Command("taskkill", "/F", "/PID", strconv.Itoa(pid))
		if err := killCmd.Run(); err != nil {
			s.logger.Warn("终止当前进程失败: %v，将使用 os.Exit", err)
			os.Exit(0)
		}
	} else {
		process, err := os.FindProcess(pid)
		if err != nil {
			s.logger.Warn("查找当前进程失败: %v，将使用 os.Exit", err)
			os.Exit(0)
		}

		if err := process.Signal(os.Interrupt); err != nil {
			s.logger.Warn("发送终止信号失败: %v，将使用 os.Exit", err)
			os.Exit(0)
		}

		time.Sleep(5 * time.Second)
		os.Exit(0)
	}

	return nil
}
