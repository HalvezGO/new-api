package operation_setting

import (
	"fmt"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
)

const RetryModelOverridesOptionKey = "RetryModelOverrides"

// RetryPolicyOverride optionally overrides the global retry policy for one
// model. A nil field falls back to the next level: channel setting, then the
// global RetryTimes / RetryInterval options.
type RetryPolicyOverride struct {
	RetryTimes    *int     `json:"retry_times,omitempty"`
	RetryInterval *int     `json:"retry_interval,omitempty"`
	RemoveParams  []string `json:"remove_params,omitempty"`
}

var (
	retryModelOverridesMutex sync.RWMutex
	retryModelOverrides      = map[string]RetryPolicyOverride{}
)

// GetRetryModelOverride returns the override configured for the exact model
// name requested by the client.
func GetRetryModelOverride(modelName string) (RetryPolicyOverride, bool) {
	retryModelOverridesMutex.RLock()
	defer retryModelOverridesMutex.RUnlock()
	override, ok := retryModelOverrides[strings.TrimSpace(modelName)]
	return override, ok
}

// RetryModelOverridesToJSONString returns the canonical JSON representation of
// the configured per-model retry overrides.
func RetryModelOverridesToJSONString() string {
	retryModelOverridesMutex.RLock()
	defer retryModelOverridesMutex.RUnlock()
	if len(retryModelOverrides) == 0 {
		return "{}"
	}
	data, err := common.Marshal(retryModelOverrides)
	if err != nil {
		common.SysError("failed to marshal retry model overrides: " + err.Error())
		return "{}"
	}
	return string(data)
}

// ValidateRetryModelOverrides checks that value is a JSON object whose entries
// only contain in-range retry_times / retry_interval fields.
func ValidateRetryModelOverrides(value string) error {
	_, err := parseRetryModelOverrides(value)
	return err
}

// UpdateRetryModelOverridesFromJSONString parses and replaces the in-memory
// per-model retry overrides.
func UpdateRetryModelOverridesFromJSONString(value string) error {
	parsed, err := parseRetryModelOverrides(value)
	if err != nil {
		return err
	}
	retryModelOverridesMutex.Lock()
	retryModelOverrides = parsed
	retryModelOverridesMutex.Unlock()
	return nil
}

func parseRetryModelOverrides(value string) (map[string]RetryPolicyOverride, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return map[string]RetryPolicyOverride{}, nil
	}

	var raw map[string]RetryPolicyOverride
	if err := common.Unmarshal([]byte(trimmed), &raw); err != nil {
		return nil, fmt.Errorf("%s must be a JSON object: %w", RetryModelOverridesOptionKey, err)
	}

	parsed := make(map[string]RetryPolicyOverride, len(raw))
	for modelName, override := range raw {
		name := strings.TrimSpace(modelName)
		if name == "" {
			return nil, fmt.Errorf("%s contains an empty model name", RetryModelOverridesOptionKey)
		}
		if override.RetryTimes == nil && override.RetryInterval == nil && len(override.RemoveParams) == 0 {
			return nil, fmt.Errorf("%s[%s] must set retry_times, retry_interval, or remove_params", RetryModelOverridesOptionKey, name)
		}
		if override.RetryTimes != nil {
			if *override.RetryTimes < 0 || *override.RetryTimes > common.MaxRetryTimes {
				return nil, fmt.Errorf("%s[%s].retry_times must be between 0 and %d", RetryModelOverridesOptionKey, name, common.MaxRetryTimes)
			}
		}
		if override.RetryInterval != nil {
			if *override.RetryInterval < 0 || *override.RetryInterval > common.MaxRetryInterval {
				return nil, fmt.Errorf("%s[%s].retry_interval must be between 0 and %d", RetryModelOverridesOptionKey, name, common.MaxRetryInterval)
			}
		}
		normalizedRemoveParams, err := dto.NormalizeParamRemovalList(override.RemoveParams)
		if err != nil {
			return nil, fmt.Errorf("%s[%s].remove_params: %w", RetryModelOverridesOptionKey, name, err)
		}
		override.RemoveParams = normalizedRemoveParams
		parsed[name] = override
	}
	return parsed, nil
}
