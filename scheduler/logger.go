package scheduler

import (
	"fmt"

	"github.com/rs/zerolog/log"
)

// zerologAdapter 把 asynq 內部的 log 導到專案統一的 zerolog，
// 格式才會跟其他元件一致。實作 asynq.Logger interface。
type zerologAdapter struct{}

func (zerologAdapter) Debug(args ...interface{}) { log.Debug().Msg(fmt.Sprint(args...)) }
func (zerologAdapter) Info(args ...interface{})  { log.Info().Msg(fmt.Sprint(args...)) }
func (zerologAdapter) Warn(args ...interface{})  { log.Warn().Msg(fmt.Sprint(args...)) }
func (zerologAdapter) Error(args ...interface{}) { log.Error().Msg(fmt.Sprint(args...)) }
func (zerologAdapter) Fatal(args ...interface{}) { log.Fatal().Msg(fmt.Sprint(args...)) }
