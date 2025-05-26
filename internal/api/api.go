package api

import (
    "database/sql"
    "context" // 👈 新增：用于 Stop() 中的 context
    "net/http"
    "strconv"
    "time"

    "pg-backup/internal/backup"
    "pg-backup/internal/config"
    "pg-backup/internal/scheduler"

    "github.com/gin-contrib/cors"
    "github.com/gin-gonic/gin"
)

type BackupRequest struct {
    Type          string `json:"type" binding:"required,oneof=local s3"`
    IncludeData   bool   `json:"includeData"`
    IncludeSchema bool   `json:"includeSchema"`
    Compression   bool   `json:"compression"`
}

type APIServer struct {
    db               *sql.DB
    config           *config.Config
    backupService    *backup.Service
    schedulerService *scheduler.Service
    router           *gin.Engine
    httpServer       *http.Server // 👈 新增字段，用于优雅关闭
}

func New(db *sql.DB, cfg *config.Config, backupService *backup.Service, schedulerService *scheduler.Service) *APIServer {
    gin.SetMode(gin.ReleaseMode)
    router := gin.New()
    router.Use(gin.Logger(), gin.Recovery())

    // CORS 配置
    router.Use(cors.New(cors.Config{
        AllowOrigins:     []string{"*"},
        AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
        AllowHeaders:     []string{"*"},
        ExposeHeaders:    []string{"Content-Length"},
        AllowCredentials: true,
        MaxAge:           12 * time.Hour,
    }))

    server := &APIServer{
        db:               db,
        config:           cfg,
        backupService:    backupService,
        schedulerService: schedulerService,
        router:           router,
    }

    server.setupRoutes()
    return server
}

// Start 启动 API 服务
func (s *APIServer) Start() error {
    addr := ":" + s.config.API.Port
    s.httpServer = &http.Server{
        Addr:    addr,
        Handler: s.router,
    }
    return s.httpServer.ListenAndServe()
}

// Stop 优雅地关闭 API 服务
func (s *APIServer) Stop() error {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    if s.httpServer != nil {
        return s.httpServer.Shutdown(ctx)
    }
    return nil
}

func (s *APIServer) setupRoutes() {
    api := s.router.Group("/api/v1")
    {
        // 备份相关路由
        api.POST("/backup", s.createBackup)
        api.GET("/backups", s.getBackupHistory)
        api.DELETE("/backups/:id", s.deleteBackup)
        api.GET("/backups/:id/download", s.downloadBackup)

        // 定时任务相关路由
        api.GET("/jobs", s.getScheduledJobs)
        api.POST("/jobs", s.createScheduledJob)
        api.PUT("/jobs/:id", s.updateScheduledJob)
        api.DELETE("/jobs/:id", s.deleteScheduledJob)
        api.POST("/jobs/:id/toggle", s.toggleScheduledJob)

        // 配置相关路由
        api.GET("/config", s.getConfigurations)
        api.PUT("/config", s.updateConfigurations)

        // 状态检查
        api.GET("/health", s.healthCheck)
        api.GET("/stats", s.getStats)
    }
}

// 备份相关处理函数
func (s *APIServer) createBackup(c *gin.Context) {
    var req BackupRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    // 异步执行备份
    go func() {
        if err := s.backupService.CreateBackup(req.IncludeData, req.IncludeSchema, req.Compression); err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
            return
        }
    }()

    c.JSON(http.StatusAccepted, gin.H{"message": "备份任务已启动"})
}

func (s *APIServer) getBackupHistory(c *gin.Context) {
    records, err := s.backupService.GetBackupHistory()
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    c.JSON(http.StatusOK, records)
}

func (s *APIServer) deleteBackup(c *gin.Context) {
    id, err := strconv.ParseInt(c.Param("id"), 10, 64)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid backup ID"})
        return
    }

    if err := s.backupService.DeleteBackup(id); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusOK, gin.H{"message": "Backup deleted successfully"})
}

func (s *APIServer) downloadBackup(c *gin.Context) {
    id, err := strconv.ParseInt(c.Param("id"), 10, 64)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid backup ID"})
        return
    }

    data, err := s.backupService.DownloadBackup(id)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.Data(http.StatusOK, "application/octet-stream", data)
}

// 定时任务相关处理函数
func (s *APIServer) getScheduledJobs(c *gin.Context) {
    jobs, err := s.schedulerService.GetJobs()
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    c.JSON(http.StatusOK, jobs)
}

func (s *APIServer) createScheduledJob(c *gin.Context) {
    var job scheduler.ScheduledJob
    if err := c.ShouldBindJSON(&job); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    if err := s.schedulerService.CreateJob(&job); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusCreated, job)
}

func (s *APIServer) updateScheduledJob(c *gin.Context) {
    // TODO: 实现更新定时任务的逻辑
    c.JSON(http.StatusOK, gin.H{"message": "Job updated"})
}

func (s *APIServer) deleteScheduledJob(c *gin.Context) {
    id, err := strconv.ParseInt(c.Param("id"), 10, 64)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid job ID"})
        return
    }

    if err := s.schedulerService.DeleteJob(id); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusOK, gin.H{"message": "Job deleted successfully"})
}

func (s *APIServer) toggleScheduledJob(c *gin.Context) {
    id, err := strconv.ParseInt(c.Param("id"), 10, 64)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid job ID"})
        return
    }

    newStatus, err := s.schedulerService.ToggleJob(id)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusOK, gin.H{"enabled": newStatus})
}

// 配置相关处理函数
func (s *APIServer) getConfigurations(c *gin.Context) {
    c.JSON(http.StatusOK, s.config)
}

func (s *APIServer) updateConfigurations(c *gin.Context) {
    var newConfig config.Config
    if err := c.ShouldBindJSON(&newConfig); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid configuration payload"})
        return
    }

    // TODO: 实现配置更新逻辑
    c.JSON(http.StatusOK, gin.H{"message": "Configuration updated"})
}

// 状态检查处理函数
func (s *APIServer) healthCheck(c *gin.Context) {
    // 检查数据库连接
    if err := s.db.Ping(); err != nil {
        c.JSON(http.StatusServiceUnavailable, gin.H{
            "status": "unhealthy",
            "error":  "Database connection failed",
        })
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "status":    "healthy",
        "timestamp": time.Now(),
        "version":   "1.0.0",
    })
}

func (s *APIServer) getStats(c *gin.Context) {
    var totalBackups, successfulBackups, failedBackups, activeJobs int

    s.db.QueryRow("SELECT COUNT(*) FROM backup_records").Scan(&totalBackups)
    s.db.QueryRow("SELECT COUNT(*) FROM backup_records WHERE status = 'completed'").Scan(&successfulBackups)
    s.db.QueryRow("SELECT COUNT(*) FROM backup_records WHERE status = 'failed'").Scan(&failedBackups)
    s.db.QueryRow("SELECT COUNT(*) FROM scheduled_jobs WHERE enabled = true").Scan(&activeJobs)

    c.JSON(http.StatusOK, gin.H{
        "totalBackups":      totalBackups,
        "successfulBackups": successfulBackups,
        "failedBackups":     failedBackups,
        "activeJobs":        activeJobs,
    })
}