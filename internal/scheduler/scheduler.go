package scheduler

import (
	"context"
	"database/sql"
	"log"
	"sync"
	"time"

	"pg-backup/internal/backup"

	"github.com/robfig/cron/v3"
)

// Logger 接口用于解耦日志实现
type Logger interface {
	Printf(format string, v ...interface{})
}

// Service 定时任务服务
type Service struct {
	db            *sql.DB
	backupService *backup.Service
	cronJobs      map[int64]*cron.Cron
	mutex         sync.RWMutex
	logger        Logger // 日志接口
	ctx           context.Context
	cancel        context.CancelFunc
}

// ScheduledJob 定义定时任务结构
type ScheduledJob struct {
	ID           int64
	Name         string
	Type         string
	Schedule     string
	ScheduleText string
	Enabled      bool
	LastRun      string
	NextRun      string
	Status       string
}

// New 创建一个新的调度服务实例
func New(db *sql.DB, backupService *backup.Service) *Service {
	ctx, cancel := context.WithCancel(context.Background())
	return &Service{
		db:            db,
		backupService: backupService,
		cronJobs:      make(map[int64]*cron.Cron),
		logger:        log.Default(),
		ctx:           ctx,
		cancel:        cancel,
	}
}

// SetLogger 设置自定义日志记录器
func (s *Service) SetLogger(logger Logger) {
	s.logger = logger
}

// Start 启动定时服务，加载已启用的任务
func (s *Service) Start() error {
	s.logger.Printf("Starting scheduler service...")
	if err := s.LoadJobs(); err != nil {
		s.logger.Printf("Failed to load scheduled jobs: %v", err)
		return err
	}
	return nil
}

// Stop 停止所有定时任务（优雅关闭）
func (s *Service) Stop() {
	s.logger.Printf("Stopping scheduler service...")

	s.cancel() // 触发 context.Done()

	s.mutex.Lock()
	defer s.mutex.Unlock()

	for id, cronJob := range s.cronJobs {
		cronJob.Stop()
		delete(s.cronJobs, id)
	}

	s.logger.Printf("Scheduler stopped.")
}

// CreateJob 创建定时任务
func (s *Service) CreateJob(job *ScheduledJob) error {
	// 验证 cron 表达式
	if _, err := cron.ParseStandard(job.Schedule); err != nil {
		return err
	}

	// 保存到数据库
	err := s.db.QueryRow(`
		INSERT INTO scheduled_jobs (name, type, schedule, schedule_text, enabled)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`, job.Name, job.Type, job.Schedule, job.ScheduleText, job.Enabled).Scan(&job.ID)

	if err != nil {
		return err
	}

	// 如果启用，添加到调度器
	if job.Enabled {
		s.addCronJob(job.ID, job.Schedule)
	}

	return nil
}

// GetJobs 获取所有定时任务
func (s *Service) GetJobs() ([]ScheduledJob, error) {
	rows, err := s.db.Query(`
		SELECT id, name, type, schedule, COALESCE(schedule_text, ''), enabled,
		       COALESCE(to_char(last_run, 'YYYY-MM-DD HH24:MI:SS'), '从未运行')
		FROM scheduled_jobs 
		ORDER BY id DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []ScheduledJob
	for rows.Next() {
		var job ScheduledJob
		err := rows.Scan(&job.ID, &job.Name, &job.Type, &job.Schedule,
			&job.ScheduleText, &job.Enabled, &job.LastRun)
		if err != nil {
			continue
		}

		// 计算下次运行时间
		if job.Enabled {
			job.Status = "active"
			if parsed, err := cron.ParseStandard(job.Schedule); err == nil {
				job.NextRun = parsed.Next(time.Now()).Format("2006-01-02 15:04:05")
			} else {
				job.NextRun = "计算失败"
			}
		} else {
			job.Status = "paused"
			job.NextRun = "已暂停"
		}

		jobs = append(jobs, job)
	}

	return jobs, nil
}

// ToggleJob 切换任务状态
func (s *Service) ToggleJob(id int64) (bool, error) {
	var enabled bool
	err := s.db.QueryRow("SELECT enabled FROM scheduled_jobs WHERE id = $1", id).Scan(&enabled)
	if err != nil {
		return false, err
	}

	newStatus := !enabled
	_, err = s.db.Exec("UPDATE scheduled_jobs SET enabled = $1 WHERE id = $2", newStatus, id)
	if err != nil {
		return false, err
	}

	s.mutex.Lock()
	if cronJob, exists := s.cronJobs[id]; exists {
		cronJob.Stop()
		delete(s.cronJobs, id)
	}
	s.mutex.Unlock()

	if newStatus {
		var schedule, jobType string
		s.db.QueryRow("SELECT schedule, type FROM scheduled_jobs WHERE id = $1", id).Scan(&schedule, &jobType)
		s.addCronJob(id, schedule)
	}

	return newStatus, nil
}

// DeleteJob 删除定时任务
func (s *Service) DeleteJob(id int64) error {
	// 停止并删除 cron 任务
	s.mutex.Lock()
	if cronJob, exists := s.cronJobs[id]; exists {
		cronJob.Stop()
		delete(s.cronJobs, id)
	}
	s.mutex.Unlock()

	// 从数据库删除
	_, err := s.db.Exec("DELETE FROM scheduled_jobs WHERE id = $1", id)
	return err
}

// LoadJobs 加载所有启用的定时任务
func (s *Service) LoadJobs() error {
	rows, err := s.db.Query("SELECT id, schedule, type FROM scheduled_jobs WHERE enabled = true")
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var id int64
		var schedule, jobType string
		if err := rows.Scan(&id, &schedule, &jobType); err == nil {
			s.addCronJob(id, schedule)
		}
	}

	return nil
}

// addCronJob 添加定时任务到调度器
func (s *Service) addCronJob(jobID int64, schedule string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	cronInstance := cron.New()
	_, err := cronInstance.AddFunc(schedule, func() {
		// 删除未使用的 ctx 声明
		// ctx := context.Background()

		// 执行备份
		if err := s.backupService.CreateBackup(true, true, true); err != nil {
			log.Printf("Scheduled backup failed for job %d: %v", jobID, err)
		}

		// 更新最后运行时间，使用 ExecContext
		ctx := context.Background()
		_, err := s.db.ExecContext(ctx, "UPDATE scheduled_jobs SET last_run = CURRENT_TIMESTAMP WHERE id = $1", jobID)
		if err != nil {
			log.Printf("Failed to update last run time for job %d: %v", jobID, err)
		}
	})

	if err != nil {
		log.Printf("Failed to add cron job %d: %v", jobID, err)
		return
	}

	cronInstance.Start()
	s.cronJobs[jobID] = cronInstance
}
