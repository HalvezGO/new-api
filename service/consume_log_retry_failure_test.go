package service

import (
	"errors"
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func appendFailure(t *testing.T, relayInfo *relaycommon.RelayInfo, retryIndex int) {
	t.Helper()
	err := types.NewErrorWithStatusCode(
		errors.New("upstream returned status 429: too many requests"),
		types.ErrorCodeBadResponseStatusCode,
		429,
	)
	AppendRetryFailure(relayInfo, types.ChannelError{
		ChannelId:   100 + retryIndex,
		ChannelName: "channel-x",
	}, err)
}

func TestAppendRetryFailureRecordFields(t *testing.T) {
	relayInfo := &relaycommon.RelayInfo{}
	relayInfo.RetryIndex = 2
	relayInfo.LastRetryWaitMs = 1500

	err := types.NewErrorWithStatusCode(
		errors.New("upstream returned status 503: service unavailable"),
		types.ErrorCodeBadResponseStatusCode,
		503,
	)
	AppendRetryFailure(relayInfo, types.ChannelError{
		ChannelId:   42,
		ChannelName: "byok-sensenova",
	}, err)

	require.Len(t, relayInfo.ConsumeLogRetryFailures, 1)
	record := relayInfo.ConsumeLogRetryFailures[0]
	assert.Equal(t, 2, record.RetryIndex)
	assert.Equal(t, 42, record.ChannelId)
	assert.Equal(t, "byok-sensenova", record.ChannelName)
	assert.Equal(t, 503, record.StatusCode)
	assert.NotEmpty(t, record.ErrorCode)
	assert.Contains(t, record.Reason, "503")
	assert.Equal(t, 1500, record.WaitedMs)
}

func TestAppendRetryFailureCapsAndTruncates(t *testing.T) {
	relayInfo := &relaycommon.RelayInfo{}
	for i := 0; i < relaycommon.MaxRetryFailureRecords+5; i++ {
		relayInfo.RetryIndex = i
		appendFailure(t, relayInfo, i)
	}

	require.Len(t, relayInfo.ConsumeLogRetryFailures, relaycommon.MaxRetryFailureRecords)
	assert.True(t, relayInfo.ConsumeLogRetryFailuresTruncated)
	assert.Equal(t, 0, relayInfo.ConsumeLogRetryFailures[0].RetryIndex)
	assert.Equal(t, relaycommon.MaxRetryFailureRecords-1, relayInfo.ConsumeLogRetryFailures[len(relayInfo.ConsumeLogRetryFailures)-1].RetryIndex)
}
