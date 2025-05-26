package app

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"pg-backup/internal/api"
	"pg-backup/internal/backup"
	"pg-backup/internal/config"
	"pg-backup/internal/scheduler"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	_ "github.com/lib/pq" // PostgreSQL driver
)

// App 是整个应用的核心控制结构
type App struct {
	cfg           *config.Config
	db            *sql.DB
	s3Client      *s3.Client
	backupService *backup.Service
	scheduler     *scheduler.Service
	apiServer     *api.APIServer
}

// Run 启动整个应用程序
func Run() {
	app := &App{}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := app.start(); err != nil {
		log.Fatalf("Application failed to start: %v", err)
	}

	<-ctx.Done()
	log.Println("Shutting down gracefully...")

	if err := app.stop(); err != nil {
		log.Printf("Error during shutdown: %v", err)
	}
}

// start 启动所有服务组件
func (a *App) start() error {
	// 加载配置
	cfg, err := config.Load("config.yaml")
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	a.cfg = cfg

	// 初始化数据库连接
	db, err := a.initDatabase()
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	a.db = db

	// 初始化 S3 客户端（如果启用）
	if cfg.Storage.Type == "s3" {
		a.s3Client = a.initS3Client()
	}

	// 初始化备份服务
	a.backupService = backup.New(a.db, a.cfg, a.s3Client)

	// 初始化定时任务服务
	a.scheduler = scheduler.New(a.db, a.backupService)
	if err := a.scheduler.Start(); err != nil {
		return fmt.Errorf("failed to start scheduler: %w", err)
	}

	// 初始化并启动 API 服务
	a.apiServer = api.New(a.db, a.cfg, a.backupService, a.scheduler)
	if err := a.apiServer.Start(); err != nil {
		return fmt.Errorf("failed to start API server: %w", err)
	}

	log.Println("All services started successfully.")
	return nil
}

// stop 关闭所有服务组件（优雅关闭）
func (a *App) stop() error {
	var errs []error

	// 停止 API 服务器（如果有 Shutdown 方法）
	if a.apiServer != nil {
		if err := a.apiServer.Stop(); err != nil {
			errs = append(errs, fmt.Errorf("API server stop error: %w", err))
		}
	}

	// 停止调度器
	if a.scheduler != nil {
		a.scheduler.Stop()
	}

	// 关闭数据库连接
	if a.db != nil {
		if err := a.db.Close(); err != nil {
			errs = append(errs, fmt.Errorf("database close error: %w", err))
		}
	}

	if len(errs) > 0 {
		return errs[0] // 返回第一个错误作为汇总
	}
	return nil
}

// initDatabase 初始化数据库连接
func (a *App) initDatabase() (*sql.DB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		a.cfg.Database.Host,
		a.cfg.Database.Port,
		a.cfg.Database.Username,
		a.cfg.Database.Password,
		a.cfg.Database.Database,
	)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}

// initS3Client 初始化 S3 客户端（示例）
func (a *App) initS3Client() *s3.Client {
	// 示例实现，请根据实际需求补充
	/*
	   awsCfg, err := awsconfig.LoadDefaultConfig(context.TODO(),
	       awsconfig.WithRegion(a.cfg.Storage.S3.Region),
	   )
	   if err != nil {
	       log.Printf("Failed to load AWS config: %v", err)
	       return nil
	   }
	   return s3.NewFromConfig(awsCfg)
	*/
	return nil
}
