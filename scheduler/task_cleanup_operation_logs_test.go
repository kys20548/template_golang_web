package scheduler

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/hibiken/asynq"
	mockdb "github.com/kys20548/template_golang_web/db/mock"
	"github.com/kys20548/template_golang_web/util"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// eqTimeApprox 比對時間參數：與期望值相差在容忍範圍內就算相等。
// cutoff 是 handler 執行當下用 time.Now() 算的，測試無法精確預測。
type eqTimeApprox struct {
	expected  time.Time
	tolerance time.Duration
}

func (m eqTimeApprox) Matches(x any) bool {
	actual, ok := x.(time.Time)
	if !ok {
		return false
	}
	diff := actual.Sub(m.expected)
	return diff > -m.tolerance && diff < m.tolerance
}

func (m eqTimeApprox) String() string {
	return fmt.Sprintf("is within %v of %v", m.tolerance, m.expected)
}

func TestHandleCleanupOperationLogs(t *testing.T) {
	retentionMonths := 3

	testCases := []struct {
		name       string
		buildStubs func(store *mockdb.MockStore)
		checkErr   func(t *testing.T, err error)
	}{
		{
			name: "OK",
			buildStubs: func(store *mockdb.MockStore) {
				cutoff := time.Now().AddDate(0, -retentionMonths, 0)
				store.EXPECT().
					DeleteOperationLogsBefore(gomock.Any(), eqTimeApprox{cutoff, 5 * time.Second}).
					Times(1).
					Return(int64(42), nil)
			},
			checkErr: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name: "DBError",
			buildStubs: func(store *mockdb.MockStore) {
				store.EXPECT().
					DeleteOperationLogsBefore(gomock.Any(), gomock.Any()).
					Times(1).
					Return(int64(0), sql.ErrConnDone)
			},
			checkErr: func(t *testing.T, err error) {
				// 要把 error 回給 asynq 才會觸發重試
				require.ErrorIs(t, err, sql.ErrConnDone)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			s := &Scheduler{
				config: util.Config{OperationLogRetentionMonths: retentionMonths},
				store:  store,
			}

			task := asynq.NewTask(TaskCleanupOperationLogs, nil)
			err := s.handleCleanupOperationLogs(context.Background(), task)
			tc.checkErr(t, err)
		})
	}
}
