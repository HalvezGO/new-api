package service

import (
	"context"
	"testing"

	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A stream that died because the caller hung up must carry the same top-level
// client_gone marker as a failed non-stream request, so the log row reads as
// "client gone" instead of an upstream problem.
func TestAppendStreamStatusMarksClientGone(t *testing.T) {
	relayInfo := &relaycommon.RelayInfo{
		IsStream:     true,
		StreamStatus: relaycommon.NewStreamStatus(),
	}
	relayInfo.StreamStatus.SetEndReason(relaycommon.StreamEndReasonClientGone, context.Canceled)

	other := model.NewLogOther()
	appendStreamStatus(relayInfo, other)

	snapshot := other.Snapshot()
	assert.Equal(t, true, snapshot["client_gone"])
	streamStatus, ok := snapshot["stream_status"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "error", streamStatus["status"])
	assert.Equal(t, "client_gone", streamStatus["end_reason"])
}

func TestAppendStreamStatusKeepsNormalEndUnmarked(t *testing.T) {
	relayInfo := &relaycommon.RelayInfo{
		IsStream:     true,
		StreamStatus: relaycommon.NewStreamStatus(),
	}
	relayInfo.StreamStatus.SetEndReason(relaycommon.StreamEndReasonDone, nil)

	other := model.NewLogOther()
	appendStreamStatus(relayInfo, other)

	assert.NotContains(t, other.Snapshot(), "client_gone")
}
