package scheduler

import (
	"context"
	"time"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog/log"
)

// TaskCleanupOperationLogs 清理過期 operation_logs 的任務類型名。
const TaskCleanupOperationLogs = "operation_log:cleanup"

// cleanupOperationLogsTask 定義清理任務的排程與 enqueue 選項。
// 這個任務冪等（重複刪同一批資料沒有副作用），掛 Unique 純粹是省掉
// 多 instance 重複執行；TTL 大於各 instance 時鐘誤差即可，不用太長。
func (s *Scheduler) cleanupOperationLogsTask() periodicTask {
	return periodicTask{
		cronSpec: s.config.OperationLogCleanupCron,
		task:     asynq.NewTask(TaskCleanupOperationLogs, nil),
		opts:     []asynq.Option{asynq.Unique(time.Minute)},
		handler:  s.handleCleanupOperationLogs,
	}
}

// handleCleanupOperationLogs 刪除超過保留期限（RETENTION_MONTHS 個月）的
// operation_logs。回傳 error 時 asynq 會自動重試；就算全部重試都失敗，
// 隔天的排程也會把積欠的資料一起清掉，所以不需要額外補償機制。
func (s *Scheduler) handleCleanupOperationLogs(ctx context.Context, task *asynq.Task) error {
	cutoff := time.Now().AddDate(0, -s.config.OperationLogRetentionMonths, 0)

	deleted, err := s.store.DeleteOperationLogsBefore(ctx, cutoff)
	if err != nil {
		log.Error().Err(err).Msg("清理 operation_logs 失敗")
		return err
	}

	log.Info().Int64("deleted", deleted).Time("cutoff", cutoff).Msg("operation_logs 清理完成")
	return nil
}
