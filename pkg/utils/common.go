package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// FormatFileSize 将字节大小转换为人类可读格式
func FormatFileSize(bytes int64) string {
	const (
		B  = 1
		KB = 1024 * B
		MB = 1024 * KB
		GB = 1024 * MB
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

// EnsureDir 确保目录存在，如果不存在则创建
func EnsureDir(path string) error {
	return os.MkdirAll(path, 0755)
}

// FileExists 检查文件是否存在
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

// GetAbsPath 获取绝对路径
func GetAbsPath(path string) (string, error) {
	return filepath.Abs(path)
}

// FormatTime 格式化时间为标准格式
func FormatTime(t time.Time) string {
	return t.Format("2006-01-02 15:04:05")
}

// ParseTime 解析标准格式的时间字符串
func ParseTime(timeStr string) (time.Time, error) {
	return time.Parse("2006-01-02 15:04:05", timeStr)
}

// SafeRemove 安全删除文件
func SafeRemove(path string) error {
	if FileExists(path) {
		return os.Remove(path)
	}
	return nil
}