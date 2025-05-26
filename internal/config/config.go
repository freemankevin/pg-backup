package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	Database DatabaseConfig `json:"database"`
	Storage  StorageConfig  `json:"storage"`
	API      APIConfig      `json:"api"`
}

type DatabaseConfig struct {
	Host     string `json:"host" binding:"required"`
	Port     int    `json:"port" binding:"required"`
	Database string `json:"database" binding:"required"`
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type StorageConfig struct {
	Type      string      `json:"type" binding:"required,oneof=local s3"` // "local" or "s3"
	Local     LocalConfig `json:"local"`
	S3        S3Config    `json:"s3"`
}

type LocalConfig struct {
	BackupPath    string `json:"backupPath" binding:"required"`
	Compression   bool   `json:"compression"`
	Retention     int    `json:"retention"`
	VerifyContent bool   `json:"verifyContent"`
}

type S3Config struct {
	Endpoint  string `json:"endpoint" binding:"required"`
	AccessKey string `json:"accessKey" binding:"required"`
	SecretKey string `json:"secretKey" binding:"required"`
	Bucket    string `json:"bucket" binding:"required"`
	Region    string `json:"region"`
}

type APIConfig struct {
	Port string `json:"port" binding:"required"`
}

// Load 从配置文件加载配置
func Load(configPath string) (*Config, error) {
	// 如果未指定配置文件路径，使用默认配置
	if configPath == "" {
		return loadDefaultConfig(), nil
	}

	// 读取配置文件
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	// 解析配置文件
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// Save 保存配置到文件
func (c *Config) Save(configPath string) error {
	// 确保配置目录存在
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return err
	}

	// 序列化配置
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	// 写入文件
	return os.WriteFile(configPath, data, 0644)
}

// loadDefaultConfig 返回默认配置
func loadDefaultConfig() *Config {
	return &Config{
		Database: DatabaseConfig{
			Host:     "localhost",
			Port:     5432,
			Database: "myapp_production",
			Username: "postgres",
			Password: "",
		},
		Storage: StorageConfig{
			Type: "local",
			Local: LocalConfig{
				BackupPath:    "/var/backups/postgresql",
				Compression:   true,
				Retention:     30,
				VerifyContent: true,
			},
			S3: S3Config{
				Region: "us-east-1",
			},
		},
		API: APIConfig{
			Port: "8080",
		},
	}
}