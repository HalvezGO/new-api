package service

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/stretchr/testify/assert"
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
	})

	common.RetryTimes = 3
	common.RetryInterval = 900

	channelWithOverrides := newRetryPolicyChannel(t, `{
		"retry_times": 7,
		"retry_interval": 1200,
		"retry_model_overrides": {
			"model-both": {"retry_times": 5, "retry_interval": 1500},
			"model-times": {"retry_times": 1},
			"model-interval": {"retry_interval": 200}
		}
	}`)
	channelWithoutOverrides := newRetryPolicyChannel(t, `{"retry_times": 9, "retry_interval": 800}`)

	t.Run("falls back to the global option without a channel", func(t *testing.T) {
		policy := ResolveRetryPolicy("model-both", nil)

		assert.Equal(t, 3, policy.RetryTimes)
		assert.Equal(t, 900, policy.RetryInterval)
	})

	t.Run("channel override wins over the global option", func(t *testing.T) {
		policy := ResolveRetryPolicy("unknown-model", channelWithOverrides)

		assert.Equal(t, 7, policy.RetryTimes)
		assert.Equal(t, 1200, policy.RetryInterval)
	})

	t.Run("channel model override wins over the channel override", func(t *testing.T) {
		policy := ResolveRetryPolicy("model-both", channelWithOverrides)

		assert.Equal(t, 5, policy.RetryTimes)
		assert.Equal(t, 1500, policy.RetryInterval)
	})

	t.Run("channel model override falls back per field to the channel override", func(t *testing.T) {
		timesPolicy := ResolveRetryPolicy("model-times", channelWithOverrides)
		assert.Equal(t, 1, timesPolicy.RetryTimes)
		assert.Equal(t, 1200, timesPolicy.RetryInterval)

		intervalPolicy := ResolveRetryPolicy("model-interval", channelWithOverrides)
		assert.Equal(t, 7, intervalPolicy.RetryTimes)
		assert.Equal(t, 200, intervalPolicy.RetryInterval)
	})

	t.Run("another channel with the same model keeps its own policy", func(t *testing.T) {
		policy := ResolveRetryPolicy("model-both", channelWithoutOverrides)

		assert.Equal(t, 9, policy.RetryTimes)
		assert.Equal(t, 800, policy.RetryInterval)
	})

	t.Run("channel model override matches the model name without the channel prefix", func(t *testing.T) {
		channelWithOverrides.SetOtherSettings(dto.ChannelOtherSettings{ModelPrefix: "byok/"})

		policy := ResolveRetryPolicy("byok/model-both", channelWithOverrides)

		assert.Equal(t, 5, policy.RetryTimes)
		assert.Equal(t, 1500, policy.RetryInterval)

		unknownPolicy := ResolveRetryPolicy("byok/unknown-model", channelWithOverrides)

		assert.Equal(t, 7, unknownPolicy.RetryTimes)
		assert.Equal(t, 1200, unknownPolicy.RetryInterval)
	})

	t.Run("stored channel values are clamped", func(t *testing.T) {
		clamped := newRetryPolicyChannel(t, `{
			"retry_times": 1000,
			"retry_interval": 999999,
			"retry_model_overrides": {
				"model-clamped": {"retry_times": 1000, "retry_interval": 999999}
			}
		}`)

		policy := ResolveRetryPolicy("model-clamped", clamped)

		assert.Equal(t, common.MaxRetryTimes, policy.RetryTimes)
		assert.Equal(t, common.MaxRetryInterval, policy.RetryInterval)
	})

	t.Run("unknown model override falls back to the channel override", func(t *testing.T) {
		policy := ResolveRetryPolicy("model-both", newRetryPolicyChannel(t, `{"retry_times": 4}`))

		assert.Equal(t, 4, policy.RetryTimes)
		assert.Equal(t, 900, policy.RetryInterval)
	})
}
