package backup

import (
	"bytes"
	"compress/gzip"
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"pg-backup/internal/config"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type Service struct {
	db       *sql.DB
	config   *config.Config
	s3Client *s3.Client
}

type BackupRecord struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Type      string    `json:"type"`
	Size      string    `json:"size"`
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
	Path      string    `json:"path"`
	Error     string    `json:"error,omitempty"`
}

func New(db *sql.DB, cfg *config.Config, s3Client *s3.Client) *Service {
	return &Service{
		db:       db,
		config:   cfg,
		s3Client: s3Client,
	}
}

// CreateBackup 创建数据库备份
func (s *Service) CreateBackup(includeData, includeSchema, compression bool) error {
	timestamp := time.Now()
	backupName := fmt.Sprintf("backup_%s", timestamp.Format("20060102_150405"))

	// 创建备份记录
	recordID, err := s.createBackupRecord(backupName, s.config.Storage.Type, "running")
	if err != nil {
		return err
	}

	// 构建 pg_dump 命令
	dumpFile := filepath.Join(os.TempDir(), backupName+".sql")
	cmd := s.buildPgDumpCommand(includeData, includeSchema, compression, dumpFile)

	// 执行备份命令
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		s.updateBackupRecord(recordID, "failed", "", "", stderr.String())
		return fmt.Errorf("pg_dump failed: %v, stderr: %s", err, stderr.String())
	}

	// 获取文件大小
	fileInfo, err := os.Stat(dumpFile)
	if err != nil {
		s.updateBackupRecord(recordID, "failed", "", "", err.Error())
		return err
	}
	if fileInfo.Size() == 0 {
		s.updateBackupRecord(recordID, "failed", "0 B", "", "pg_dump generated an empty file")
		return fmt.Errorf("empty backup file generated")
	}

	// 内容校验
	if s.config.Storage.Local.VerifyContent {
		var content []byte
		if strings.HasSuffix(dumpFile, ".gz") {
			file, err := os.Open(dumpFile)
			if err == nil {
				defer file.Close()
				gzr, err := gzip.NewReader(file)
				if err == nil {
					defer gzr.Close()
					content, err = io.ReadAll(gzr)
				}
			}
		} else {
			content, err = os.ReadFile(dumpFile)
		}

		if err == nil {
			if !bytes.Contains(content, []byte("CREATE")) && !bytes.Contains(content, []byte("INSERT")) {
				s.updateBackupRecord(recordID, "failed", formatFileSize(fileInfo.Size()), "", "backup file contains no CREATE or INSERT")
				return fmt.Errorf("backup file content validation failed")
			}
		}
	}

	size := formatFileSize(fileInfo.Size())
	var finalPath string

	// 根据备份类型处理文件
	switch s.config.Storage.Type {
	case "local":
		finalPath, err = s.handleLocalBackup(dumpFile, backupName)
	case "s3":
		finalPath, err = s.handleS3Backup(context.Background(), dumpFile, backupName)
	}

	// 清理临时文件
	os.Remove(dumpFile)

	if err != nil {
		s.updateBackupRecord(recordID, "failed", size, "", err.Error())
		return err
	}

	// 更新备份记录为成功
	s.updateBackupRecord(recordID, "completed", size, finalPath, "")
	return nil
}

// GetBackupHistory 获取备份历史记录
func (s *Service) GetBackupHistory() ([]BackupRecord, error) {
	rows, err := s.db.Query(`
		SELECT id, name, type, COALESCE(size, ''), status, timestamp, COALESCE(path, ''), COALESCE(error, '')
		FROM backup_records 
		ORDER BY timestamp DESC 
		LIMIT 100
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []BackupRecord
	for rows.Next() {
		var record BackupRecord
		err := rows.Scan(&record.ID, &record.Name, &record.Type, &record.Size,
			&record.Status, &record.Timestamp, &record.Path, &record.Error)
		if err != nil {
			continue
		}
		records = append(records, record)
	}

	return records, nil
}

// DeleteBackup 删除备份
func (s *Service) DeleteBackup(id int64) error {
	_, err := s.db.Exec("DELETE FROM backup_records WHERE id = $1", id)
	return err
}

// DownloadBackup 下载备份文件
func (s *Service) DownloadBackup(id int64) ([]byte, error) {
	// TODO: 实现下载逻辑
	return nil, nil
}

// 内部辅助方法
func (s *Service) buildPgDumpCommand(includeData, includeSchema, compression bool, outputFile string) *exec.Cmd {
	args := []string{
		"-h", s.config.Database.Host,
		"-p", strconv.Itoa(s.config.Database.Port),
		"-U", s.config.Database.Username,
		"-d", s.config.Database.Database,
		"-f", outputFile,
		"--verbose",
	}

	if !includeData {
		args = append(args, "--schema-only")
	}
	if !includeSchema {
		args = append(args, "--data-only")
	}
	if compression {
		args = append(args, "--compress=6")
		// 如果使用压缩，修改输出文件扩展名
		outputFile = strings.Replace(outputFile, ".sql", ".sql.gz", 1)
		args[len(args)-3] = outputFile // 更新 -f 参数
	}

	cmd := exec.Command("pg_dump", args...)
	cmd.Env = append(os.Environ(), fmt.Sprintf("PGPASSWORD=%s", s.config.Database.Password))

	return cmd
}

func (s *Service) handleLocalBackup(sourceFile, backupName string) (string, error) {
	// 确保备份目录存在
	if err := os.MkdirAll(s.config.Storage.Local.BackupPath, 0755); err != nil {
		return "", err
	}

	finalPath := filepath.Join(s.config.Storage.Local.BackupPath, filepath.Base(sourceFile))

	// 移动文件到最终位置
	if err := os.Rename(sourceFile, finalPath); err != nil {
		return "", err
	}

	// 异步清理旧备份
	if s.config.Storage.Local.Retention > 0 {
		go s.cleanupOldBackups()
	}

	return finalPath, nil
}

func (s *Service) handleS3Backup(ctx context.Context, sourceFile, backupName string) (string, error) {
	if s.s3Client == nil {
		return "", fmt.Errorf("S3 client not configured")
	}

	file, err := os.Open(sourceFile)
	if err != nil {
		return "", err
	}
	defer file.Close()

	key := fmt.Sprintf("postgresql-backups/%s", filepath.Base(sourceFile))

	_, err = s.s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.config.Storage.S3.Bucket),
		Key:    aws.String(key),
		Body:   file,
	})

	if err != nil {
		return "", err
	}

	return fmt.Sprintf("s3://%s/%s", s.config.Storage.S3.Bucket, key), nil
}

func (s *Service) createBackupRecord(name, backupType, status string) (int64, error) {
	var id int64
	err := s.db.QueryRow(`
		INSERT INTO backup_records (name, type, status)
		VALUES ($1, $2, $3)
		RETURNING id
	`, name, backupType, status).Scan(&id)
	return id, err
}

func (s *Service) updateBackupRecord(id int64, status, size, path, errorMsg string) {
	s.db.Exec(`
		UPDATE backup_records 
		SET status = $1, size = $2, path = $3, error = $4
		WHERE id = $5
	`, status, size, path, errorMsg, id)
}

func (s *Service) cleanupOldBackups() {
	cutoff := time.Now().AddDate(0, 0, -s.config.Storage.Local.Retention)
	pattern := filepath.Join(s.config.Storage.Local.BackupPath, "backup_*.sql*")

	matches, _ := filepath.Glob(pattern)
	for _, file := range matches {
		if info, err := os.Stat(file); err == nil && info.ModTime().Before(cutoff) {
			os.Remove(file)
		}
	}
}

func formatFileSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
