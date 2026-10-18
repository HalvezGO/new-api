package service

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func newStubContext() *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	c.Set("use_channel", []string{"12", "37"})
	return c
}

func TestBuildFailedLogOtherFields(t *testing.T) {
	relayInfo := &relaycommon.RelayInfo{}
	relayInfo.RetryIndex = 1
	AppendRetryFailure(relayInfo, newStubChannelError(), newStubNewAPIError(429))

	other := buildFailedLogOther(newStubContext(), relayInfo, nil)

	assert.Equal(t, false, other["in_progress"])
	assert.Equal(t, "/v1/chat/completions", other["request_path"])
	assert.Equal(t, "status_code=429, upstream returned status: too many requests", other["failure_reason"])
	adminInfo, ok := other["admin_info"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, []string{"12", "37"}, adminInfo["use_channel"])
	assert.Equal(t, 2, adminInfo["retry_attempts"])
	assert.Len(t, other["retry_failures"], 1)
}

func TestBuildFailedLogOtherWithoutRetries(t *testing.T) {
	other := buildFailedLogOther(newStubContext(), &relaycommon.RelayInfo{}, newStubNewAPIError(503))

	assert.Contains(t, other, "failure_reason")
	assert.NotContains(t, other, "retry_failures")
	assert.Contains(t, other, "request_path")
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

func TestMergeRetryFailuresStampsRetryAttempts(t *testing.T) {
	relayInfo := &relaycommon.RelayInfo{}
	relayInfo.RetryIndex = 2
	AppendRetryFailure(relayInfo, newStubChannelError(), newStubNewAPIError(429))
	// Existing admin info (use_channel) must survive the merge.
	other := map[string]interface{}{
		"admin_info": map[string]interface{}{"use_channel": []int{12, 37}},
	}
	mergeRetryFailures(other, relayInfo)

	adminInfo, ok := other["admin_info"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, 3, adminInfo["retry_attempts"])
	assert.Equal(t, []int{12, 37}, adminInfo["use_channel"])
}

func TestMergeRetryFailuresLeavesSingleAttemptAlone(t *testing.T) {
	relayInfo := &relaycommon.RelayInfo{}
	relayInfo.RetryIndex = 0
	AppendRetryFailure(relayInfo, newStubChannelError(), newStubNewAPIError(429))
	other := map[string]interface{}{}
	mergeRetryFailures(other, relayInfo)

	require.Len(t, other["retry_failures"], 1)
	assert.NotContains(t, other, "admin_info")
}
