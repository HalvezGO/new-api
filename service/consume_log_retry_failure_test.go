package service

import (
	"errors"
	"testing"
	"time"

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
	relayInfo := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelIsMultiKey:    true,
			ChannelMultiKeyIndex: 3,
		},
	}
	relayInfo.RetryIndex = 2
	relayInfo.LastRetryWaitMs = 1500

	err := types.NewErrorWithStatusCode(
		errors.New("upstream returned status 503: service unavailable"),
		types.ErrorCodeBadResponseStatusCode,
		503,
	)
	before := time.Now().Unix()
	// The first attempt reuses the distributed channel snapshot, which may not
	// carry the multi-key flag; the relay info still knows the selected key.
	AppendRetryFailure(relayInfo, types.ChannelError{
		ChannelId:   42,
		ChannelName: "byok-sensenova",
		IsMultiKey:  false,
		UsingKey:    "sk-1234567890abcdef",
	}, err)

	require.Len(t, relayInfo.ConsumeLogRetryFailures, 1)
	record := relayInfo.ConsumeLogRetryFailures[0]
	assert.Equal(t, 2, record.RetryIndex)
	assert.Equal(t, 42, record.ChannelId)
	assert.Equal(t, "byok-sensenova", record.ChannelName)
	require.NotNil(t, record.KeyIndex)
	assert.Equal(t, 3, *record.KeyIndex)
	assert.Equal(t, "sk-1**********cdef", record.KeyMasked)
	assert.Equal(t, 503, record.StatusCode)
	assert.NotEmpty(t, record.ErrorCode)
	assert.Contains(t, record.Reason, "503")
	assert.Equal(t, 1500, record.WaitedMs)
	assert.GreaterOrEqual(t, record.AttemptedAt, before)
}

func TestAppendRetryFailureOmitsKeyIndexForSingleKeyChannel(t *testing.T) {
	relayInfo := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{ChannelMultiKeyIndex: 4},
	}
	AppendRetryFailure(relayInfo, types.ChannelError{
		ChannelId:  7,
		IsMultiKey: false,
		UsingKey:   "sk-single-secret",
	}, types.NewErrorWithStatusCode(errors.New("upstream 429"), types.ErrorCodeBadResponseStatusCode, 429))

	require.Len(t, relayInfo.ConsumeLogRetryFailures, 1)
	record := relayInfo.ConsumeLogRetryFailures[0]
	assert.Nil(t, record.KeyIndex)
	assert.Equal(t, "sk-s**********cret", record.KeyMasked)
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
