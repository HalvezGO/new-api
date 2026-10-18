package model

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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
	updated, err := UpdateConsumeLogFinal(testGinContext(), id, finalParams)
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
	updated, err = UpdateConsumeLogFinal(testGinContext(), id, finalParams)
	require.NoError(t, err)
	assert.False(t, updated)
	require.NoError(t, LOG_DB.First(&reloaded, id).Error)
	assert.EqualValues(t, LogStatusCompleted, reloaded.Status)
	assert.Equal(t, 500, reloaded.Quota)
}

func TestUpdateConsumeLogFinalMissingRowReturnsFalse(t *testing.T) {
	truncateTables(t)

	updated, err := UpdateConsumeLogFinal(testGinContext(), 424242, RecordConsumeLogParams{Quota: 10})
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
	updated, err := UpdateConsumeLogFinal(testGinContext(), completed, RecordConsumeLogParams{Quota: 1})
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
