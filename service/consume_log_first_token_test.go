package service

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newStreamingRelayInfo(t *testing.T) *relaycommon.RelayInfo {
	t.Helper()
	relayInfo, err := relaycommon.GenRelayInfo(sweepTestContext(), types.RelayFormatOpenAI, nil, nil)
	require.NoError(t, err)
	relayInfo.IsStream = true
	// Pending rows use the request start as their anchor, and the first-token
	// latency is measured from it.
	relayInfo.StartTime = time.Now().Add(-1500 * time.Millisecond)
	return relayInfo
}

// While a stream is still running, the first-token latency lands on the pending
// row so the logs list can show progress instead of a placeholder.
func TestFirstTokenUpdatesPendingConsumeLog(t *testing.T) {
	model.DB.Exec("DELETE FROM logs")
	relayInfo := newStreamingRelayInfo(t)

	id := RecordPendingConsumeLog(sweepTestContext(), relayInfo)
	require.NotZero(t, id)
	require.NotNil(t, relayInfo.OnFirstResponse)

	relayInfo.SetFirstResponseTime()

	require.Eventually(t, func() bool {
		var reloaded model.Log
		if err := model.DB.First(&reloaded, id).Error; err != nil {
			return false
		}
		var other map[string]interface{}
		if err := common.UnmarshalJsonStr(reloaded.Other, &other); err != nil {
			return false
		}
		frt, ok := other["frt"].(float64)
		return ok && frt >= 1000
	}, 2*time.Second, 10*time.Millisecond, "pending row should carry the first-token latency")
}

// Non-streaming requests have no early first token, so they must not pay for the
// extra write.
func TestFirstTokenHookOnlyForStreaming(t *testing.T) {
	model.DB.Exec("DELETE FROM logs")
	relayInfo, err := relaycommon.GenRelayInfo(sweepTestContext(), types.RelayFormatOpenAI, nil, nil)
	require.NoError(t, err)
	require.False(t, relayInfo.IsStream)

	id := RecordPendingConsumeLog(sweepTestContext(), relayInfo)
	require.NotZero(t, id)
	assert.Nil(t, relayInfo.OnFirstResponse)
}

// A stream that produced a first token and then failed keeps that latency: it is
// the same number a completed row would show.
func TestFailedConsumeLogKeepsFirstTokenTime(t *testing.T) {
	model.DB.Exec("DELETE FROM logs")
	relayInfo := newStreamingRelayInfo(t)

	id := RecordPendingConsumeLog(sweepTestContext(), relayInfo)
	require.NotZero(t, id)
	relayInfo.SetFirstResponseTime()

	MarkConsumeLogFailed(sweepTestContext(), relayInfo, nil)

	var reloaded model.Log
	require.NoError(t, model.DB.First(&reloaded, id).Error)
	assert.EqualValues(t, model.LogStatusFailed, reloaded.Status)

	var other map[string]interface{}
	require.NoError(t, common.UnmarshalJsonStr(reloaded.Other, &other))
	frt, ok := other["frt"].(float64)
	require.True(t, ok, "failed row should keep the first-token latency")
	assert.GreaterOrEqual(t, frt, 1000.0)
}
