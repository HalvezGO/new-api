package service

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

// RetryPolicy is the effective retry configuration for one retry attempt.
// RetryTimes uses the same unit as the global RetryTimes option: the maximum
// number of retries, so the request may make RetryTimes+1 attempts.
type RetryPolicy struct {
	RetryTimes    int
	RetryInterval int // milliseconds
}

// ResolveRetryPolicy resolves the retry settings with priority
// model > channel > global. Pass a nil channel before one is selected; the
// model and global levels are always available.
//
// Channel settings are only consulted when the model override does not define
// the field, so a model can pin the retry count while inheriting the channel's
// interval (or vice versa).
func ResolveRetryPolicy(modelName string, channel *model.Channel) RetryPolicy {
	policy := RetryPolicy{
		RetryTimes:    common.RetryTimes,
		RetryInterval: common.RetryInterval,
	}

	if channel != nil {
		// Model overrides are keyed by the model name shown in the channel's
		// Models list. Clients call prefixed names, so strip the channel prefix
		// before looking the override up.
		modelName = strings.TrimPrefix(modelName, channel.GetOtherSettings().ModelPrefix)
		channelSetting := channel.GetSetting()
		if channelSetting.RetryTimes != nil {
			policy.RetryTimes = *channelSetting.RetryTimes
		}
		if channelSetting.RetryInterval != nil {
			policy.RetryInterval = *channelSetting.RetryInterval
		}
	}

	if override, ok := operation_setting.GetRetryModelOverride(modelName); ok {
		if override.RetryTimes != nil {
			policy.RetryTimes = *override.RetryTimes
		}
		if override.RetryInterval != nil {
			policy.RetryInterval = *override.RetryInterval
		}
	}

	// Channel settings come from stored JSON, so clamp defensively instead of
	// trusting the value that went through validation.
	policy.RetryTimes = min(max(policy.RetryTimes, 0), common.MaxRetryTimes)
	policy.RetryInterval = min(max(policy.RetryInterval, 0), common.MaxRetryInterval)
	return policy
}
