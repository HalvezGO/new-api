package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/require"
)

func TestValidateOptionValueRetryBounds(t *testing.T) {
	require.NoError(t, validateOptionValue("RetryTimes", "0"))
	require.NoError(t, validateOptionValue("RetryTimes", "100"))
	require.Error(t, validateOptionValue("RetryTimes", "-1"))
	require.Error(t, validateOptionValue("RetryTimes", "101"))
	require.Error(t, validateOptionValue("RetryTimes", "abc"))

	require.NoError(t, validateOptionValue("RetryInterval", "0"))
	require.NoError(t, validateOptionValue("RetryInterval", "30000"))
	require.Error(t, validateOptionValue("RetryInterval", "-1"))
	require.Error(t, validateOptionValue("RetryInterval", "30001"))
}

// The database load path bypasses validateOptionValue, so out-of-range legacy
// values must be clamped instead of silently disabling retries (negative
// RetryTimes) or holding a request open for an unbounded time.
func TestUpdateOptionMapClampsRetryOptions(t *testing.T) {
	prevMap := common.OptionMap
	prevRetryTimes := common.RetryTimes
	prevRetryInterval := common.RetryInterval
	prevRanges := operation_setting.RetryIntervalStatusCodeRanges
	t.Cleanup(func() {
		common.OptionMap = prevMap
		common.RetryTimes = prevRetryTimes
		common.RetryInterval = prevRetryInterval
		operation_setting.RetryIntervalStatusCodeRanges = prevRanges
	})
	common.OptionMap = map[string]string{}

	require.NoError(t, updateOptionMap("RetryTimes", "150"))
	require.Equal(t, common.MaxRetryTimes, common.RetryTimes)
	require.NoError(t, updateOptionMap("RetryTimes", "-3"))
	require.Equal(t, 0, common.RetryTimes)
	require.NoError(t, updateOptionMap("RetryTimes", "7"))
	require.Equal(t, 7, common.RetryTimes)

	require.NoError(t, updateOptionMap("RetryInterval", "90000"))
	require.Equal(t, common.MaxRetryInterval, common.RetryInterval)
	require.NoError(t, updateOptionMap("RetryInterval", "abc"))
	require.Equal(t, 0, common.RetryInterval)
	require.NoError(t, updateOptionMap("RetryInterval", "2500"))
	require.Equal(t, 2500, common.RetryInterval)

	require.NoError(t, updateOptionMap("RetryIntervalStatusCodes", "429,503"))
	require.True(t, operation_setting.ShouldUseRetryInterval(429))
	require.True(t, operation_setting.ShouldUseRetryInterval(503))
	require.False(t, operation_setting.ShouldUseRetryInterval(500))
}
