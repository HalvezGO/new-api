package service

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sweepTestContext() *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	return c
}

// ClickHouse has no UPDATE statement, so the sweep must skip it entirely
// instead of failing (or, worse, silently marking rows via another driver).
func TestSweepStaleInProgressLogsSkipsClickHouse(t *testing.T) {
	originalLogType := common.LogDatabaseType()
	t.Cleanup(func() { common.SetLogDatabaseType(originalLogType) })

	model.DB.Exec("DELETE FROM logs")
	stale, err := model.RecordPendingConsumeLog(sweepTestContext(), 1, model.RecordConsumeLogParams{
		ModelName: "ch-sweep-model",
		Other:     map[string]interface{}{"in_progress": true},
	})
	require.NoError(t, err)
	require.NotZero(t, stale)
	require.NoError(t, model.DB.Model(&model.Log{}).Where("id = ?", stale).
		Update("created_at", time.Now().Add(-24*time.Hour).Unix()).Error)

	common.SetLogDatabaseType(common.DatabaseTypeClickHouse)
	sweepStaleInProgressLogs()

	var reloaded model.Log
	require.NoError(t, model.DB.First(&reloaded, stale).Error)
	assert.EqualValues(t, model.LogStatusInProgress, reloaded.Status,
		"ClickHouse log databases must not be swept")
}
