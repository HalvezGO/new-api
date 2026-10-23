package service

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newRetryPolicyChannel(t *testing.T, setting string) *model.Channel {
	t.Helper()
	return &model.Channel{Setting: &setting}
}

func TestResolveRetryPolicy(t *testing.T) {
	originalTimes := common.RetryTimes
	originalInterval := common.RetryInterval
	t.Cleanup(func() {
		common.RetryTimes = originalTimes
		common.RetryInterval = originalInterval
		require.NoError(t, operation_setting.UpdateRetryModelOverridesFromJSONString("{}"))
	})
	require.NoError(t, operation_setting.UpdateRetryModelOverridesFromJSONString(`{
		"model-both": {"retry_times": 5, "retry_interval": 1500},
		"model-times": {"retry_times": 1},
		"model-interval": {"retry_interval": 200}
	}`))

	common.RetryTimes = 3
	common.RetryInterval = 900

	t.Run("falls back to the global option without overrides", func(t *testing.T) {
		policy := ResolveRetryPolicy("unknown-model", nil)

		assert.Equal(t, 3, policy.RetryTimes)
		assert.Equal(t, 900, policy.RetryInterval)
	})

	t.Run("channel override wins over the global option", func(t *testing.T) {
		channel := newRetryPolicyChannel(t, `{"retry_times": 7, "retry_interval": 1200}`)

		policy := ResolveRetryPolicy("unknown-model", channel)

		assert.Equal(t, 7, policy.RetryTimes)
		assert.Equal(t, 1200, policy.RetryInterval)
	})

	t.Run("model override wins over the channel override", func(t *testing.T) {
		channel := newRetryPolicyChannel(t, `{"retry_times": 7, "retry_interval": 1200}`)

		policy := ResolveRetryPolicy("model-both", channel)

		assert.Equal(t, 5, policy.RetryTimes)
		assert.Equal(t, 1500, policy.RetryInterval)
	})

	t.Run("model override falls back per field to the channel override", func(t *testing.T) {
		channel := newRetryPolicyChannel(t, `{"retry_times": 7, "retry_interval": 1200}`)

		timesPolicy := ResolveRetryPolicy("model-times", channel)
		assert.Equal(t, 1, timesPolicy.RetryTimes)
		assert.Equal(t, 1200, timesPolicy.RetryInterval)

		intervalPolicy := ResolveRetryPolicy("model-interval", channel)
		assert.Equal(t, 7, intervalPolicy.RetryTimes)
		assert.Equal(t, 200, intervalPolicy.RetryInterval)
	})

	t.Run("model override without a channel falls back per field to the global option", func(t *testing.T) {
		policy := ResolveRetryPolicy("model-times", nil)

		assert.Equal(t, 1, policy.RetryTimes)
		assert.Equal(t, 900, policy.RetryInterval)
	})

	t.Run("model override matches the model name without the channel prefix", func(t *testing.T) {
		channel := newRetryPolicyChannel(t, `{"retry_times": 7, "retry_interval": 1200}`)
		channel.SetOtherSettings(dto.ChannelOtherSettings{ModelPrefix: "byok/"})

		policy := ResolveRetryPolicy("byok/model-both", channel)

		assert.Equal(t, 5, policy.RetryTimes)
		assert.Equal(t, 1500, policy.RetryInterval)

		unknownPolicy := ResolveRetryPolicy("byok/unknown-model", channel)

		assert.Equal(t, 7, unknownPolicy.RetryTimes)
		assert.Equal(t, 1200, unknownPolicy.RetryInterval)
	})

	t.Run("stored channel values are clamped", func(t *testing.T) {
		channel := newRetryPolicyChannel(t, `{"retry_times": 1000, "retry_interval": 999999}`)

		policy := ResolveRetryPolicy("unknown-model", channel)

		assert.Equal(t, common.MaxRetryTimes, policy.RetryTimes)
		assert.Equal(t, common.MaxRetryInterval, policy.RetryInterval)
	})
}
