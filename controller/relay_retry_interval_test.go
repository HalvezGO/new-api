package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
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
	origInterval := common.RetryInterval
	origRanges := operation_setting.RetryIntervalStatusCodeRanges
	t.Cleanup(func() {
		common.RetryInterval = origInterval
		operation_setting.RetryIntervalStatusCodeRanges = origRanges
	})
	operation_setting.RetryIntervalStatusCodeRanges = []operation_setting.StatusCodeRange{{Start: 429, End: 429}}

	t.Run("zero interval keeps the legacy zero-delay retry", func(t *testing.T) {
		common.RetryInterval = 0
		require.True(t, waitBeforeRetry(newRetryIntervalTestContext(t, nil), nil, 429, 3))
	})

	t.Run("no remaining retries skips the wait", func(t *testing.T) {
		common.RetryInterval = 10000
		require.True(t, waitBeforeRetry(newRetryIntervalTestContext(t, nil), nil, 429, 0))
	})

	t.Run("unlisted status code skips the wait", func(t *testing.T) {
		common.RetryInterval = 10000
		require.True(t, waitBeforeRetry(newRetryIntervalTestContext(t, nil), nil, 500, 3))
	})

	t.Run("completed wait continues retrying", func(t *testing.T) {
		common.RetryInterval = 1
		require.True(t, waitBeforeRetry(newRetryIntervalTestContext(t, nil), nil, 429, 3))
	})

	t.Run("cancelled client aborts the wait", func(t *testing.T) {
		common.RetryInterval = 30000
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		require.False(t, waitBeforeRetry(newRetryIntervalTestContext(t, ctx), nil, 429, 3))
	})
}
