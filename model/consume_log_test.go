package model

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testGinContext() *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	return c
}

func pendingParams() RecordConsumeLogParams {
	return RecordConsumeLogParams{
		ModelName:        "test-model",
		TokenName:        "test-token",
		TokenId:          1,
		IsStream:         false,
		Group:            "default",
		ChannelId:        0,
		UseTimeSeconds:   999, // must be forced to 0 for pending rows
		Quota:            999,
		PromptTokens:     999,
		CompletionTokens: 999,
		Other: map[string]interface{}{
			"in_progress": true,
		},
	}
}

func TestRecordPendingConsumeLogDefaults(t *testing.T) {
	truncateTables(t)

	params := pendingParams()
	id, err := RecordPendingConsumeLog(testGinContext(), 1, params)
	require.NoError(t, err)
	require.NotZero(t, id)

	var reloaded Log
	require.NoError(t, LOG_DB.First(&reloaded, id).Error)
	assert.EqualValues(t, LogStatusInProgress, reloaded.Status)
	assert.Equal(t, 0, reloaded.Quota)
	assert.Equal(t, 0, reloaded.PromptTokens)
	assert.Equal(t, 0, reloaded.CompletionTokens)
	assert.Equal(t, 0, reloaded.UseTime)
	assert.Equal(t, LogTypeConsume, reloaded.Type)
	assert.Equal(t, "test-model", reloaded.ModelName)
	assert.Contains(t, reloaded.Other, "\"in_progress\":true")
}

// The finalize export must be keyed by the request's user, not a placeholder,
// otherwise per-user usage-data views (filtered on user_id) go empty.
func TestUpdateConsumeLogFinalExportsQuotaDataWithUserId(t *testing.T) {
	truncateTables(t)

	prevExport := common.DataExportEnabled
	t.Cleanup(func() { common.DataExportEnabled = prevExport })
	common.DataExportEnabled = true

	id, err := RecordPendingConsumeLog(testGinContext(), 7, pendingParams())
	require.NoError(t, err)

	// Isolate the export cache before finalizing so the assertion below sees
	// exactly the row this test produces.
	CacheQuotaDataLock.Lock()
	CacheQuotaData = make(map[string]*QuotaData)
	CacheQuotaDataLock.Unlock()

	requestStart := time.Now().Add(-90 * time.Second).Unix()
	updated, err := UpdateConsumeLogFinal(testGinContext(), 7, id, RecordConsumeLogParams{
		ModelName:        "test-model",
		Group:            "default",
		Quota:            321,
		PromptTokens:     10,
		CompletionTokens: 5,
		CreatedAt:        requestStart,
		Other:            map[string]interface{}{},
	})
	require.NoError(t, err)
	require.True(t, updated)

	SaveQuotaDataCache()

	var exported []QuotaData
	require.NoError(t, LOG_DB.Where("model_name = ?", "test-model").Find(&exported).Error)
	require.Len(t, exported, 1)
	assert.Equal(t, 7, exported[0].UserID)
	assert.Equal(t, requestStart-requestStart%3600, exported[0].CreatedAt)
	assert.Equal(t, 321, exported[0].Quota)
}

func TestUpdateConsumeLogFinalGuard(t *testing.T) {
	truncateTables(t)

	id, err := RecordPendingConsumeLog(testGinContext(), 1, pendingParams())
	require.NoError(t, err)

	finalParams := RecordConsumeLogParams{
		ChannelId:        7,
		PromptTokens:     100,
		CompletionTokens: 50,
		ModelName:        "test-model",
		TokenName:        "test-token",
		TokenId:          1,
		Quota:            500,
		UseTimeSeconds:   3,
		IsStream:         true,
		Group:            "default",
	}
	updated, err := UpdateConsumeLogFinal(testGinContext(), 1, id, finalParams)
	require.NoError(t, err)
	assert.True(t, updated)

	var reloaded Log
	require.NoError(t, LOG_DB.First(&reloaded, id).Error)
	assert.EqualValues(t, LogStatusCompleted, reloaded.Status)
	assert.Equal(t, 500, reloaded.Quota)
	assert.Equal(t, 100, reloaded.PromptTokens)
	assert.Equal(t, 50, reloaded.CompletionTokens)
	assert.Equal(t, 3, reloaded.UseTime)
	assert.Equal(t, 7, reloaded.ChannelId)

	// A second finalize (both success and failure paths raced) must be a no-op.
	updated, err = UpdateConsumeLogFinal(testGinContext(), 1, id, finalParams)
	require.NoError(t, err)
	assert.False(t, updated)
	require.NoError(t, LOG_DB.First(&reloaded, id).Error)
	assert.EqualValues(t, LogStatusCompleted, reloaded.Status)
	assert.Equal(t, 500, reloaded.Quota)
}

func TestUpdateConsumeLogFinalMissingRowReturnsFalse(t *testing.T) {
	truncateTables(t)

	updated, err := UpdateConsumeLogFinal(testGinContext(), 1, 424242, RecordConsumeLogParams{Quota: 10})
	require.NoError(t, err)
	assert.False(t, updated)
}

func TestUpdateConsumeLogFailedKeepsTokensZero(t *testing.T) {
	truncateTables(t)

	id, err := RecordPendingConsumeLog(testGinContext(), 1, pendingParams())
	require.NoError(t, err)

	updated, err := UpdateConsumeLogFailed(testGinContext(), id, 12, 7, map[string]interface{}{
		"failure_reason": "upstream 429",
	})
	require.NoError(t, err)
	assert.True(t, updated)

	var reloaded Log
	require.NoError(t, LOG_DB.First(&reloaded, id).Error)
	assert.EqualValues(t, LogStatusFailed, reloaded.Status)
	assert.Equal(t, 12, reloaded.UseTime)
	assert.Equal(t, 7, reloaded.ChannelId)
	assert.Equal(t, 0, reloaded.Quota)
	assert.Equal(t, 0, reloaded.PromptTokens)
	assert.Equal(t, 0, reloaded.CompletionTokens)
	assert.Contains(t, reloaded.Other, "failure_reason")
}

func TestSweepStaleInProgressLogsOnlyStalePending(t *testing.T) {
	truncateTables(t)

	now := time.Now().Unix()

	stale, err := RecordPendingConsumeLog(testGinContext(), 1, pendingParams())
	require.NoError(t, err)
	require.NoError(t, LOG_DB.Model(&Log{}).Where("id = ?", stale).
		Update("created_at", now-3600).Error)

	fresh, err := RecordPendingConsumeLog(testGinContext(), 2, pendingParams())
	require.NoError(t, err)
	require.NoError(t, LOG_DB.Model(&Log{}).Where("id = ?", fresh).
		Update("created_at", now-10).Error)

	completed, err := RecordPendingConsumeLog(testGinContext(), 3, pendingParams())
	require.NoError(t, err)
	updated, err := UpdateConsumeLogFinal(testGinContext(), 3, completed, RecordConsumeLogParams{Quota: 1})
	require.NoError(t, err)
	require.True(t, updated)
	require.NoError(t, LOG_DB.Model(&Log{}).Where("id = ?", completed).
		Update("created_at", now-7200).Error)

	failed, err := RecordPendingConsumeLog(testGinContext(), 4, pendingParams())
	require.NoError(t, err)
	updated, err = UpdateConsumeLogFailed(testGinContext(), failed, 1, 0, nil)
	require.NoError(t, err)
	require.True(t, updated)
	require.NoError(t, LOG_DB.Model(&Log{}).Where("id = ?", failed).
		Update("created_at", now-7200).Error)

	swept, err := SweepStaleInProgressLogs(now - 1800)
	require.NoError(t, err)
	assert.EqualValues(t, 1, swept)

	var staleRow, freshRow, completedRow, failedRow Log
	require.NoError(t, LOG_DB.First(&staleRow, stale).Error)
	require.NoError(t, LOG_DB.First(&freshRow, fresh).Error)
	require.NoError(t, LOG_DB.First(&completedRow, completed).Error)
	require.NoError(t, LOG_DB.First(&failedRow, failed).Error)
	assert.EqualValues(t, LogStatusTimeout, staleRow.Status)
	assert.EqualValues(t, LogStatusInProgress, freshRow.Status)
	assert.EqualValues(t, LogStatusCompleted, completedRow.Status)
	assert.EqualValues(t, LogStatusFailed, failedRow.Status)
}
