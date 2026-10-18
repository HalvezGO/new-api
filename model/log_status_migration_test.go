package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A log row inserted without explicitly setting Status (i.e. every legacy row
// and every row written by the non-pending paths) must read back as completed,
// otherwise stats/aggregations that filter on status would treat history as
// pending.
func TestLegacyLogRowDefaultsToCompletedStatus(t *testing.T) {
	truncateTables(t)

	row := &Log{
		UserId:    1,
		Type:      LogTypeConsume,
		ModelName: "legacy-model",
		Quota:     0,
	}
	require.NoError(t, createLog(row))

	var reloaded Log
	require.NoError(t, LOG_DB.First(&reloaded, row.Id).Error)
	assert.EqualValues(t, LogStatusCompleted, reloaded.Status)
}
