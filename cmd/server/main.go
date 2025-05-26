package main

import (
	"context"
	"database/sql"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"pg-backup/internal/api"
	"pg-backup/internal/backup"
	"pg-backup/internal/config"
	"pg-backup/internal/scheduler"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	_ "github.com/lib/pq"
)

func main() {
	// 解析命令行参数
	configPath := flag.String("config", "", "配置文件路径")
	flag.Parse()

	// 加载配置
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 连接数据库
	db, err := sql.Open("postgres", "host=localhost port=5432 user=postgres dbname=backup_manager sslmode=disable")
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// 配置连接池
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)

	// 初始化 S3 客户端
	var s3Client *s3.Client
	if cfg.Storage.Type == "s3" && cfg.Storage.S3.AccessKey != "" {
		s3Cfg, err := awsconfig.LoadDefaultConfig(context.TODO(),
			awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
				cfg.Storage.S3.AccessKey,
				cfg.Storage.S3.SecretKey,
				"",
			)),
			awsconfig.WithRegion(cfg.Storage.S3.Region),
		)
		if err != nil {
			log.Printf("Failed to load S3 config: %v", err)
		} else {
			s3Client = s3.NewFromConfig(s3Cfg, func(o *s3.Options) {
				if cfg.Storage.S3.Endpoint != "" {
					o.BaseEndpoint = aws.String(cfg.Storage.S3.Endpoint)
					o.UsePathStyle = true
				}
			})
		}
	}

	// 初始化服务
	backupService := backup.New(db, cfg, s3Client)
	schedulerService := scheduler.New(db, backupService)
	apiServer := api.New(db, cfg, backupService, schedulerService)

	// 加载定时任务
	if err := schedulerService.LoadJobs(); err != nil {
		log.Printf("Failed to load scheduled jobs: %v", err)
	}

	// 启动 API 服务器
	go func() {
		if err := apiServer.Start(); err != nil {
			log.Fatalf("Failed to start API server: %v", err)
		}
	}()

	// 等待中断信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down...")
}