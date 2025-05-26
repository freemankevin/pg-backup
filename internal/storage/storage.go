package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"pg-backup/internal/config"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type Storage interface {
	Store(ctx context.Context, key string, data io.Reader) error
	Retrieve(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
	List(ctx context.Context, prefix string) ([]string, error)
}

type Service struct {
	config   *config.StorageConfig
	s3Client *s3.Client
}

func New(cfg *config.StorageConfig) *Service {
	return &Service{
		config: cfg,
	}
}

func (s *Service) SetS3Client(client *s3.Client) {
	s.s3Client = client
}

// Store 存储文件
func (s *Service) Store(ctx context.Context, key string, data io.Reader) error {
	switch s.config.Type {
	case "local":
		return s.storeLocal(key, data)
	case "s3":
		return s.storeS3(ctx, key, data)
	default:
		return fmt.Errorf("unsupported storage type: %s", s.config.Type)
	}
}

// Retrieve 获取文件
func (s *Service) Retrieve(ctx context.Context, key string) (io.ReadCloser, error) {
	switch s.config.Type {
	case "local":
		return s.retrieveLocal(key)
	case "s3":
		return s.retrieveS3(ctx, key)
	default:
		return nil, fmt.Errorf("unsupported storage type: %s", s.config.Type)
	}
}

// Delete 删除文件
func (s *Service) Delete(ctx context.Context, key string) error {
	switch s.config.Type {
	case "local":
		return s.deleteLocal(key)
	case "s3":
		return s.deleteS3(ctx, key)
	default:
		return fmt.Errorf("unsupported storage type: %s", s.config.Type)
	}
}

// List 列出文件
func (s *Service) List(ctx context.Context, prefix string) ([]string, error) {
	switch s.config.Type {
	case "local":
		return s.listLocal(prefix)
	case "s3":
		return s.listS3(ctx, prefix)
	default:
		return nil, fmt.Errorf("unsupported storage type: %s", s.config.Type)
	}
}

// 本地存储实现
func (s *Service) storeLocal(key string, data io.Reader) error {
	fullPath := filepath.Join(s.config.Local.BackupPath, key)
	
	// 确保目录存在
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	file, err := os.Create(fullPath)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, data)
	return err
}

func (s *Service) retrieveLocal(key string) (io.ReadCloser, error) {
	fullPath := filepath.Join(s.config.Local.BackupPath, key)
	return os.Open(fullPath)
}

func (s *Service) deleteLocal(key string) error {
	fullPath := filepath.Join(s.config.Local.BackupPath, key)
	return os.Remove(fullPath)
}

func (s *Service) listLocal(prefix string) ([]string, error) {
	pattern := filepath.Join(s.config.Local.BackupPath, prefix+"*")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}

	var result []string
	for _, match := range matches {
		rel, err := filepath.Rel(s.config.Local.BackupPath, match)
		if err == nil {
			result = append(result, rel)
		}
	}
	return result, nil
}

// S3存储实现
func (s *Service) storeS3(ctx context.Context, key string, data io.Reader) error {
	if s.s3Client == nil {
		return fmt.Errorf("S3 client not initialized")
	}

	_, err := s.s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.config.S3.Bucket),
		Key:    aws.String(key),
		Body:   data,
	})
	return err
}

func (s *Service) retrieveS3(ctx context.Context, key string) (io.ReadCloser, error) {
	if s.s3Client == nil {
		return nil, fmt.Errorf("S3 client not initialized")
	}

	result, err := s.s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.config.S3.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}
	return result.Body, nil
}

func (s *Service) deleteS3(ctx context.Context, key string) error {
	if s.s3Client == nil {
		return fmt.Errorf("S3 client not initialized")
	}

	_, err := s.s3Client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.config.S3.Bucket),
		Key:    aws.String(key),
	})
	return err
}

func (s *Service) listS3(ctx context.Context, prefix string) ([]string, error) {
	if s.s3Client == nil {
		return nil, fmt.Errorf("S3 client not initialized")
	}

	result, err := s.s3Client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.config.S3.Bucket),
		Prefix: aws.String(prefix),
	})
	if err != nil {
		return nil, err
	}
	var keys []string
	for _, obj := range result.Contents {
		keys = append(keys, *obj.Key)
	}
	return keys, nil
}