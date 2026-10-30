package service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testLogOther(values map[string]any) *model.LogOther {
	other := model.NewLogOther()
	for key, value := range values {
		if key == "admin_info" {
			if admin, ok := value.(map[string]any); ok {
				other.MergeAdmin(admin)
			}
			continue
		}
		other.SetPublic(key, value)
	}
	return other
}

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
	snapshot := other.Snapshot()

	assert.Equal(t, false, snapshot["in_progress"])
	assert.Equal(t, "/v1/chat/completions", snapshot["request_path"])
	assert.Equal(t, "status_code=429, upstream returned status: too many requests", snapshot["failure_reason"])
	adminInfo, ok := snapshot["admin_info"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, []string{"12", "37"}, adminInfo["use_channel"])
	assert.Equal(t, 2, adminInfo["retry_attempts"])
	assert.Len(t, snapshot["retry_failures"], 1)
}

func TestBuildFailedLogOtherWithoutRetries(t *testing.T) {
	other := buildFailedLogOther(newStubContext(), &relaycommon.RelayInfo{}, newStubNewAPIError(503))
	snapshot := other.Snapshot()

	assert.Contains(t, snapshot, "failure_reason")
	assert.NotContains(t, snapshot, "retry_failures")
	assert.Contains(t, snapshot, "request_path")
}

// A canceled client request must be distinguishable from an upstream failure in
// the log row, so it is not read as a provider problem.
func TestBuildFailedLogOtherMarksClientGone(t *testing.T) {
	clientGoneErr := types.NewErrorWithStatusCode(
		context.Canceled,
		types.ErrorCodeDoRequestFailed,
		500,
	)

	other := buildFailedLogOther(newStubContext(), &relaycommon.RelayInfo{}, clientGoneErr)
	snapshot := other.Snapshot()

	assert.Equal(t, true, snapshot["client_gone"])
	assert.Contains(t, snapshot, "failure_reason")
}

func TestMergeRetryFailuresIntoOther(t *testing.T) {
	relayInfo := &relaycommon.RelayInfo{}
	relayInfo.RetryIndex = 1
	relayInfo.LastRetryWaitMs = 800
	AppendRetryFailure(relayInfo, newStubChannelError(), newStubNewAPIError(429))

	other := testLogOther(map[string]any{"model_ratio": 1.0})
	mergeRetryFailures(other, relayInfo)
	snapshot := other.Snapshot()
	assert.Len(t, snapshot["retry_failures"], 1)
	assert.NotContains(t, snapshot, "retry_failures_truncated")
}

func TestMergeRetryFailuresSkipsEmpty(t *testing.T) {
	other := testLogOther(map[string]any{"model_ratio": 1.0})
	mergeRetryFailures(other, &relaycommon.RelayInfo{})
	assert.NotContains(t, other.Snapshot(), "retry_failures")
}

func TestMergeRetryFailuresFlagsTruncated(t *testing.T) {
	relayInfo := &relaycommon.RelayInfo{}
	for i := 0; i < relaycommon.MaxRetryFailureRecords+1; i++ {
		relayInfo.RetryIndex = i
		AppendRetryFailure(relayInfo, newStubChannelError(), newStubNewAPIError(429))
	}
	other := model.NewLogOther()
	mergeRetryFailures(other, relayInfo)
	snapshot := other.Snapshot()
	assert.Len(t, snapshot["retry_failures"], relaycommon.MaxRetryFailureRecords)
	assert.Equal(t, true, snapshot["retry_failures_truncated"])
}

func TestMergeRetryFailuresStampsRetryAttempts(t *testing.T) {
	relayInfo := &relaycommon.RelayInfo{}
	relayInfo.RetryIndex = 2
	AppendRetryFailure(relayInfo, newStubChannelError(), newStubNewAPIError(429))
	// Existing admin info (use_channel) must survive the merge.
	other := testLogOther(map[string]any{
		"admin_info": map[string]any{"use_channel": []int{12, 37}},
	})
	mergeRetryFailures(other, relayInfo)

	snapshot := other.Snapshot()
	adminInfo, ok := snapshot["admin_info"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, 3, adminInfo["retry_attempts"])
	assert.Equal(t, []int{12, 37}, adminInfo["use_channel"])
}

func TestMergeRetryFailuresLeavesSingleAttemptAlone(t *testing.T) {
	relayInfo := &relaycommon.RelayInfo{}
	relayInfo.RetryIndex = 0
	AppendRetryFailure(relayInfo, newStubChannelError(), newStubNewAPIError(429))
	other := model.NewLogOther()
	mergeRetryFailures(other, relayInfo)

	snapshot := other.Snapshot()
	require.Len(t, snapshot["retry_failures"], 1)
	assert.NotContains(t, snapshot, "admin_info")
}
