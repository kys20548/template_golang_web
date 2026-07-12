// Package scheduler 提供基於 asynq 的排程與背景任務處理。
//
// 兩個角色（都跑在每個 instance 裡）：
//   - asynq.Scheduler：cron 時間到，把任務 enqueue 進 Redis
//   - asynq.Server（worker）：從 Redis 取出任務執行
//
// 多 instance 部署時每台的 Scheduler 都會在同一時間 enqueue，
// 靠 asynq.Unique 以 (queue, type, payload) 在 Redis 上鎖去重：
// 搶輸的那台拿到 ErrDuplicateTask（預期行為，記 info 即可），
// 任務最終只會被其中一個 worker 執行一次。
package scheduler

import (
	"errors"
	"time"

	"github.com/hibiken/asynq"
	db "github.com/kys20548/template_golang_web/db/sqlc"
	"github.com/kys20548/template_golang_web/util"
	"github.com/rs/zerolog/log"
)

// periodicTask 是一個排程任務的完整定義：什麼時候跑（cronSpec）、
// enqueue 時帶什麼選項（opts，例如要不要 Unique 去重、TTL 多長，由任務自己決定）、
// worker 撿到後執行誰（handler）。
type periodicTask struct {
	cronSpec string
	task     *asynq.Task
	opts     []asynq.Option
	handler  asynq.HandlerFunc
}

type Scheduler struct {
	config    util.Config
	store     db.Store
	scheduler *asynq.Scheduler
	server    *asynq.Server
}

func New(config util.Config, store db.Store) *Scheduler {
	redisOpt := asynq.RedisClientOpt{Addr: config.RedisAddress}

	scheduler := asynq.NewScheduler(redisOpt, &asynq.SchedulerOpts{
		// cron 表達式用本地時區解讀（asynq 預設 UTC，「凌晨」會差 8 小時）
		Location: time.Local,
		Logger:   zerologAdapter{},
		// enqueue 結果統一在這裡記 log
		PostEnqueueFunc: func(info *asynq.TaskInfo, err error) {
			if err != nil {
				if errors.Is(err, asynq.ErrDuplicateTask) {
					log.Info().Msg("排程任務已由其他 instance 排入，跳過（unique 去重）")
					return
				}
				log.Error().Err(err).Msg("排程任務 enqueue 失敗")
				return
			}
			log.Info().Str("task", info.Type).Str("task_id", info.ID).Msg("排程任務已排入 queue")
		},
	})

	server := asynq.NewServer(redisOpt, asynq.Config{
		Logger: zerologAdapter{},
	})

	return &Scheduler{
		config:    config,
		store:     store,
		scheduler: scheduler,
		server:    server,
	}
}

// periodicTasks 集中列出所有排程任務。
// 新增任務：開一個 task_xxx.go 定義 periodicTask + handler，然後在這裡加一行。
func (s *Scheduler) periodicTasks() []periodicTask {
	return []periodicTask{
		s.cleanupOperationLogsTask(),
	}
}

// Start 註冊排程與任務 handler 並啟動（非阻塞）。
func (s *Scheduler) Start() error {
	mux := asynq.NewServeMux()

	for _, pt := range s.periodicTasks() {
		if _, err := s.scheduler.Register(pt.cronSpec, pt.task, pt.opts...); err != nil {
			return err
		}
		mux.HandleFunc(pt.task.Type(), pt.handler)
	}

	if err := s.server.Start(mux); err != nil {
		return err
	}
	if err := s.scheduler.Start(); err != nil {
		s.server.Shutdown()
		return err
	}

	log.Info().Str("cron", s.config.OperationLogCleanupCron).Msg("scheduler 已啟動")
	return nil
}

// Shutdown 優雅關閉：先停 scheduler（不再產生新任務），
// 再停 worker（等進行中的任務跑完）。
func (s *Scheduler) Shutdown() {
	s.scheduler.Shutdown()
	s.server.Shutdown()
	log.Info().Msg("scheduler 已安全終止")
}
