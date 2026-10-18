package service

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The pending row is written before any channel is selected, and ChannelMeta is
// only populated later by InitChannelMeta inside the relay handler. Reading the
// embedded ChannelId there dereferences a nil pointer and panics the whole
// request, which is what a live deployment hit.
func TestRecordPendingConsumeLogWithoutChannelMeta(t *testing.T) {
	model.DB.Exec("DELETE FROM logs")

	relayInfo := &relaycommon.RelayInfo{
		UserId:          1,
		OriginModelName: "nil-channel-meta-model",
		UsingGroup:      "default",
	}
	require.Nil(t, relayInfo.ChannelMeta)

	var id int
	require.NotPanics(t, func() {
		id = RecordPendingConsumeLog(sweepTestContext(), relayInfo)
	})
	require.NotZero(t, id)
	assert.Equal(t, id, relayInfo.ConsumeLogId)

	var reloaded model.Log
	require.NoError(t, model.DB.First(&reloaded, id).Error)
	assert.EqualValues(t, model.LogStatusInProgress, reloaded.Status)
	assert.Equal(t, 0, reloaded.ChannelId)
}

// A request can fail before InitChannelMeta ever runs (e.g. no channel matched),
// and the deferred failure handler still has to finalize the row.
func TestMarkConsumeLogFailedWithoutChannelMeta(t *testing.T) {
	model.DB.Exec("DELETE FROM logs")

	id, err := model.RecordPendingConsumeLog(sweepTestContext(), 1, model.RecordConsumeLogParams{
		ModelName: "failed-nil-channel-meta-model",
		Other:     map[string]interface{}{"in_progress": true},
	})
	require.NoError(t, err)
	require.NotZero(t, id)

	relayInfo := &relaycommon.RelayInfo{ConsumeLogId: id}
	require.Nil(t, relayInfo.ChannelMeta)

	require.NotPanics(t, func() {
		MarkConsumeLogFailed(sweepTestContext(), relayInfo, nil)
	})

	var reloaded model.Log
	require.NoError(t, model.DB.First(&reloaded, id).Error)
	assert.EqualValues(t, model.LogStatusFailed, reloaded.Status)
}
