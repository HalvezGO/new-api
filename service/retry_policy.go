package service

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

// RetryPolicy is the effective retry configuration for one retry attempt.
// RetryTimes uses the same unit as the global RetryTimes option: the maximum
// number of retries, so the request may make RetryTimes+1 attempts.
type RetryPolicy struct {
	RetryTimes    int
	RetryInterval int // milliseconds
}

// ResolveRetryPolicy resolves the retry settings with priority
// channel model > channel > global. Pass a nil channel before one is selected;
// only the global level is available then.
//
// A channel's per-model override is keyed by the model name in that channel's
// Models list, so it never affects another channel serving the same name.
// Each override field falls back independently to the channel setting, then
// the global option.
func ResolveRetryPolicy(modelName string, channel *model.Channel) RetryPolicy {
	policy := RetryPolicy{
		RetryTimes:    common.RetryTimes,
		RetryInterval: common.RetryInterval,
	}

	if channel != nil {
		// Clients call prefixed names, while the override is keyed by the
		// model name shown in the channel's Models list.
		modelName = strings.TrimPrefix(modelName, channel.GetOtherSettings().ModelPrefix)
		channelSetting := channel.GetSetting()
		if channelSetting.RetryTimes != nil {
			policy.RetryTimes = *channelSetting.RetryTimes
		}
		if channelSetting.RetryInterval != nil {
			policy.RetryInterval = *channelSetting.RetryInterval
		}
		if override, ok := channelSetting.RetryModelOverrides[modelName]; ok {
			if override.RetryTimes != nil {
				policy.RetryTimes = *override.RetryTimes
			}
			if override.RetryInterval != nil {
				policy.RetryInterval = *override.RetryInterval
			}
		}
	}

	// Channel settings come from stored JSON, so clamp defensively instead of
	// trusting the value that went through validation.
	policy.RetryTimes = min(max(policy.RetryTimes, 0), common.MaxRetryTimes)
	policy.RetryInterval = min(max(policy.RetryInterval, 0), common.MaxRetryInterval)
	return policy
}
