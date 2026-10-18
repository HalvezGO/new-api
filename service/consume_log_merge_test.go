package service

import (
	"errors"
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/stretchr/testify/assert"
)

func newStubChannelError() types.ChannelError {
	return types.ChannelError{
		ChannelId:   7,
		ChannelName: "stub-channel",
	}
}

func newStubNewAPIError(statusCode int) *types.NewAPIError {
	return types.NewErrorWithStatusCode(
		errors.New("upstream returned status: too many requests"),
		types.ErrorCodeBadResponseStatusCode,
		statusCode,
	)
}

func TestMergeRetryFailuresIntoOther(t *testing.T) {
	relayInfo := &relaycommon.RelayInfo{}
	relayInfo.RetryIndex = 1
	relayInfo.LastRetryWaitMs = 800
	AppendRetryFailure(relayInfo, newStubChannelError(), newStubNewAPIError(429))

	other := map[string]interface{}{"model_ratio": 1.0}
	mergeRetryFailures(other, relayInfo)
	assert.Len(t, other["retry_failures"], 1)
	assert.NotContains(t, other, "retry_failures_truncated")
}

func TestMergeRetryFailuresSkipsEmpty(t *testing.T) {
	other := map[string]interface{}{"model_ratio": 1.0}
	mergeRetryFailures(other, &relaycommon.RelayInfo{})
	assert.NotContains(t, other, "retry_failures")
}

func TestMergeRetryFailuresFlagsTruncated(t *testing.T) {
	relayInfo := &relaycommon.RelayInfo{}
	for i := 0; i < relaycommon.MaxRetryFailureRecords+1; i++ {
		relayInfo.RetryIndex = i
		AppendRetryFailure(relayInfo, newStubChannelError(), newStubNewAPIError(429))
	}
	other := map[string]interface{}{}
	mergeRetryFailures(other, relayInfo)
	assert.Len(t, other["retry_failures"], relaycommon.MaxRetryFailureRecords)
	assert.Equal(t, true, other["retry_failures_truncated"])
}
