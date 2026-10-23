package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newRetryIntervalTestContext(t *testing.T, ctx context.Context) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	if ctx != nil {
		req = req.WithContext(ctx)
	}
	c.Request = req
	return c
}

func TestWaitBeforeRetry(t *testing.T) {
	origRanges := operation_setting.RetryIntervalStatusCodeRanges
	t.Cleanup(func() {
		operation_setting.RetryIntervalStatusCodeRanges = origRanges
	})
	operation_setting.RetryIntervalStatusCodeRanges = []operation_setting.StatusCodeRange{{Start: 429, End: 429}}

	t.Run("zero interval keeps the legacy zero-delay retry", func(t *testing.T) {
		require.True(t, waitBeforeRetry(newRetryIntervalTestContext(t, nil), nil, 429, 3, 0))
	})

	t.Run("no remaining retries skips the wait", func(t *testing.T) {
		require.True(t, waitBeforeRetry(newRetryIntervalTestContext(t, nil), nil, 429, 0, 10000))
	})

	t.Run("unlisted status code skips the wait", func(t *testing.T) {
		require.True(t, waitBeforeRetry(newRetryIntervalTestContext(t, nil), nil, 500, 3, 10000))
	})

	t.Run("skipped wait clears a previously recorded wait", func(t *testing.T) {
		// A wait recorded before an earlier attempt must not survive a skipped
		// wait, or the next failure summary claims a pause that never happened.
		relayInfo := &relaycommon.RelayInfo{}
		relayInfo.LastRetryWaitMs = 5000

		require.True(t, waitBeforeRetry(newRetryIntervalTestContext(t, nil), relayInfo, 500, 3, 10000))
		require.Zero(t, relayInfo.LastRetryWaitMs)
	})

	t.Run("completed wait continues retrying", func(t *testing.T) {
		require.True(t, waitBeforeRetry(newRetryIntervalTestContext(t, nil), nil, 429, 3, 1))
	})

	t.Run("uses the resolved interval and records it", func(t *testing.T) {
		relayInfo := &relaycommon.RelayInfo{}
		require.True(t, waitBeforeRetry(newRetryIntervalTestContext(t, nil), relayInfo, 429, 3, 1))
		require.Equal(t, 1, relayInfo.LastRetryWaitMs)
	})

	t.Run("cancelled client aborts the wait", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		require.False(t, waitBeforeRetry(newRetryIntervalTestContext(t, ctx), nil, 429, 3, 30000))
	})
}
