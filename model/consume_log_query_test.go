package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetUserLogsStatusFilter(t *testing.T) {
	truncateTables(t)

	pendingId, err := RecordPendingConsumeLog(testGinContext(), 1, RecordConsumeLogParams{ModelName: "m1"})
	require.NoError(t, err)
	require.NotZero(t, pendingId)

	completedId, err := RecordPendingConsumeLog(testGinContext(), 1, RecordConsumeLogParams{ModelName: "m2"})
	require.NoError(t, err)
	updated, err := UpdateConsumeLogFinal(testGinContext(), 1, completedId, RecordConsumeLogParams{
		ModelName: "m2",
		Quota:     10,
		Other:     map[string]interface{}{},
	})
	require.NoError(t, err)
	require.True(t, updated)

	all, total, err := GetUserLogs(1, LogTypeConsume, 0, 0, "", "", 0, 100, "", "", "", -1)
	require.NoError(t, err)
	assert.EqualValues(t, 2, total)
	assert.Len(t, all, 2)

	pendingOnly, total, err := GetUserLogs(1, LogTypeConsume, 0, 0, "", "", 0, 100, "", "", "", LogStatusInProgress)
	require.NoError(t, err)
	assert.EqualValues(t, 1, total)
	require.Len(t, pendingOnly, 1)
	assert.EqualValues(t, LogStatusInProgress, pendingOnly[0].Status)

	completedOnly, total, err := GetUserLogs(1, LogTypeConsume, 0, 0, "", "", 0, 100, "", "", "", LogStatusCompleted)
	require.NoError(t, err)
	assert.EqualValues(t, 1, total)
	require.Len(t, completedOnly, 1)
	assert.EqualValues(t, LogStatusCompleted, completedOnly[0].Status)
}

func TestSumUsedQuotaExcludesNonFinalLogs(t *testing.T) {
	truncateTables(t)

	pendingId, err := RecordPendingConsumeLog(testGinContext(), 1, RecordConsumeLogParams{ModelName: "m1", Group: "default"})
	require.NoError(t, err)
	require.NotZero(t, pendingId)

	completedId, err := RecordPendingConsumeLog(testGinContext(), 1, RecordConsumeLogParams{ModelName: "m2", Group: "default"})
	require.NoError(t, err)
	updated, err := UpdateConsumeLogFinal(testGinContext(), 1, completedId, RecordConsumeLogParams{
		ModelName: "m2",
		Group:     "default",
		Quota:     100,
		Other:     map[string]interface{}{},
	})
	require.NoError(t, err)
	require.True(t, updated)

	stat, err := SumUsedQuota(LogTypeConsume, 0, 0, "", "", "", 0, "default")
	require.NoError(t, err)
	assert.Equal(t, 100, stat.Quota)
}

func TestUpdateOptionMapInProgressLogEnabled(t *testing.T) {
	prev := common.InProgressLogEnabled
	t.Cleanup(func() { common.InProgressLogEnabled = prev })

	previousMap := common.OptionMap
	t.Cleanup(func() { common.OptionMap = previousMap })
	common.OptionMap = map[string]string{}

	require.NoError(t, updateOptionMap("InProgressLogEnabled", "false"))
	assert.False(t, common.InProgressLogEnabled)
	require.NoError(t, updateOptionMap("InProgressLogEnabled", "true"))
	assert.True(t, common.InProgressLogEnabled)
}
