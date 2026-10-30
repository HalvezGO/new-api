package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A first-token update can race the finalize of a short stream: the status guard
// must make the late write a no-op instead of replacing the settled payload.
func TestUpdateInProgressConsumeLogOtherSkipsSettledRow(t *testing.T) {
	truncateTables(t)

	id, err := RecordPendingConsumeLog(testGinContext(), 1, RecordConsumeLogParams{
		ModelName: "first-token-model",
		Other:     testLogOther(map[string]any{"in_progress": true}),
	})
	require.NoError(t, err)
	require.NotZero(t, id)

	updated, err := UpdateInProgressConsumeLogOther(id, testLogOther(map[string]any{
		"in_progress": true,
		"frt":         1500.0,
	}))
	require.NoError(t, err)
	require.True(t, updated)

	settled, err := UpdateConsumeLogFinal(testGinContext(), 1, id, RecordConsumeLogParams{
		ModelName: "first-token-model",
		Other:     testLogOther(map[string]any{"in_progress": false, "frt": 1600.0}),
	})
	require.NoError(t, err)
	require.True(t, settled)

	updated, err = UpdateInProgressConsumeLogOther(id, testLogOther(map[string]any{
		"in_progress": true,
		"frt":         9000.0,
	}))
	require.NoError(t, err)
	assert.False(t, updated, "a settled row must never be rewritten by a late first-token update")

	var reloaded Log
	require.NoError(t, LOG_DB.First(&reloaded, id).Error)
	assert.EqualValues(t, LogStatusCompleted, reloaded.Status)

	var other map[string]interface{}
	require.NoError(t, common.UnmarshalJsonStr(reloaded.Other, &other))
	assert.EqualValues(t, 1600, other["frt"])
}
