package service

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
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
		Other:     testLogOther(map[string]any{"in_progress": true}),
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

func TestOrphanedInProgressLogIDs(t *testing.T) {
	logs := []model.Log{
		{Id: 1, Other: `{"admin_info":{"node_name":"self"}}`},
		{Id: 2, Other: `{"admin_info":{"node_name":"dead-node"}}`},
		{Id: 3, Other: `{"admin_info":{"node_name":"live-node"}}`},
		{Id: 4, Other: `{"in_progress":true}`},
		{Id: 5, Other: ""},
	}

	t.Run("single node recovers every row that predates the process", func(t *testing.T) {
		assert.ElementsMatch(t, []int{1, 2, 3, 4, 5}, orphanedInProgressLogIDs(logs, "self", nil))
	})

	t.Run("multi node leaves live siblings and legacy rows alone", func(t *testing.T) {
		assert.ElementsMatch(t, []int{1, 2},
			orphanedInProgressLogIDs(logs, "self", map[string]bool{"live-node": true}))
	})

	t.Run("without a node name only dead owners are recovered", func(t *testing.T) {
		assert.ElementsMatch(t, []int{1, 2},
			orphanedInProgressLogIDs(logs, "", map[string]bool{"live-node": true}))
	})
}

// Recovery after a crash hinges on the pending row naming its serving node;
// without that marker a restarted process can only wait for the age-based sweep.
func TestRecordPendingConsumeLogRecordsServingNode(t *testing.T) {
	model.DB.Exec("DELETE FROM logs")
	originalNode := common.NodeName
	common.NodeName = "pending-node"
	t.Cleanup(func() { common.NodeName = originalNode })

	id := RecordPendingConsumeLog(sweepTestContext(), &relaycommon.RelayInfo{
		UserId:          1,
		OriginModelName: "pending-node-model",
	})
	require.NotZero(t, id)

	var reloaded model.Log
	require.NoError(t, model.DB.First(&reloaded, id).Error)
	assert.Equal(t, "pending-node", pendingConsumeLogNodeName(reloaded.Other))
}

func recordRecoveryTestRow(t *testing.T, other *model.LogOther) int {
	t.Helper()
	logID, err := model.RecordPendingConsumeLog(sweepTestContext(), 1, model.RecordConsumeLogParams{
		ModelName: "recovery-model",
		Other:     other,
	})
	require.NoError(t, err)
	require.NotZero(t, logID)
	// Rows from a previous run predate this process.
	require.NoError(t, model.DB.Model(&model.Log{}).Where("id = ?", logID).
		Update("created_at", common.StartTime-60).Error)
	return logID
}

func TestRecoverOrphanedInProgressLogsMarksRowsFromThisNodesPastRun(t *testing.T) {
	model.DB.Exec("DELETE FROM logs")
	originalNode := common.NodeName
	common.NodeName = "recovery-node"
	t.Cleanup(func() { common.NodeName = originalNode })

	logID := recordRecoveryTestRow(t, testLogOther(map[string]any{
		"admin_info": map[string]any{"node_name": common.NodeName},
	}))

	recoverOrphanedInProgressLogs()

	var reloaded model.Log
	require.NoError(t, model.DB.First(&reloaded, logID).Error)
	assert.EqualValues(t, model.LogStatusTimeout, reloaded.Status)
}

func TestRecoverOrphanedInProgressLogsKeepsLiveNodeRows(t *testing.T) {
	model.DB.Exec("DELETE FROM logs")
	model.DB.Exec("DELETE FROM system_instances")
	require.NoError(t, model.UpsertSystemInstance("live-node", nil, common.StartTime, common.GetTimestamp()))

	logID := recordRecoveryTestRow(t, testLogOther(map[string]any{
		"admin_info": map[string]any{"node_name": "live-node"},
	}))

	recoverOrphanedInProgressLogs()

	var reloaded model.Log
	require.NoError(t, model.DB.First(&reloaded, logID).Error)
	assert.EqualValues(t, model.LogStatusInProgress, reloaded.Status)
}

func TestRecoverOrphanedInProgressLogsMarksRowsFromStaleNodes(t *testing.T) {
	// A node that crashed without restarting keeps its registry row; only the
	// heartbeat going stale tells recovery that it can no longer be serving.
	model.DB.Exec("DELETE FROM logs")
	model.DB.Exec("DELETE FROM system_instances")
	staleSeen := common.GetTimestamp() - model.SystemInstanceStaleAfterSeconds - 60
	require.NoError(t, model.UpsertSystemInstance("crashed-node", nil, staleSeen, staleSeen))

	logID := recordRecoveryTestRow(t, testLogOther(map[string]any{
		"admin_info": map[string]any{"node_name": "crashed-node"},
	}))

	recoverOrphanedInProgressLogs()

	var reloaded model.Log
	require.NoError(t, model.DB.First(&reloaded, logID).Error)
	assert.EqualValues(t, model.LogStatusTimeout, reloaded.Status)
}

func TestRecoverOrphanedInProgressLogsKeepsRowsFromCurrentRun(t *testing.T) {
	// A row created after this process started may belong to a request that is
	// still in flight, even when it carries this node's name.
	model.DB.Exec("DELETE FROM logs")
	originalNode := common.NodeName
	common.NodeName = "recovery-node"
	t.Cleanup(func() { common.NodeName = originalNode })

	logID, err := model.RecordPendingConsumeLog(sweepTestContext(), 1, model.RecordConsumeLogParams{
		ModelName: "recovery-live-model",
		Other:     testLogOther(map[string]any{"admin_info": map[string]any{"node_name": common.NodeName}}),
	})
	require.NoError(t, err)
	require.NotZero(t, logID)

	recoverOrphanedInProgressLogs()

	var reloaded model.Log
	require.NoError(t, model.DB.First(&reloaded, logID).Error)
	assert.EqualValues(t, model.LogStatusInProgress, reloaded.Status)
}
