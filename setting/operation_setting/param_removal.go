package operation_setting

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
)

const (
	ParamRemovalListOptionKey                = "ParamRemovalList"
	ParamRemovalBlacklistTTLSecondsOptionKey = "ParamRemovalBlacklistTTLSeconds"

	defaultParamRemovalBlacklistTTLSeconds = 3600
	minParamRemovalBlacklistTTLSeconds     = 60
	maxParamRemovalBlacklistTTLSeconds     = 86400
)

var (
	paramRemovalMutex               sync.RWMutex
	globalParamRemovalList          = []string{}
	paramRemovalBlacklistTTLSeconds = defaultParamRemovalBlacklistTTLSeconds
)

func GetGlobalParamRemovalList() []string {
	paramRemovalMutex.RLock()
	defer paramRemovalMutex.RUnlock()
	return append([]string(nil), globalParamRemovalList...)
}

func ParamRemovalListToJSONString() string {
	paramRemovalMutex.RLock()
	list := append([]string(nil), globalParamRemovalList...)
	paramRemovalMutex.RUnlock()
	if len(list) == 0 {
		return "[]"
	}
	data, err := common.Marshal(list)
	if err != nil {
		common.SysError("failed to marshal parameter removal list: " + err.Error())
		return "[]"
	}
	return string(data)
}

func ValidateParamRemovalList(value string) error {
	_, err := parseParamRemovalList(value)
	return err
}

func UpdateParamRemovalListFromJSONString(value string) error {
	parsed, err := parseParamRemovalList(value)
	if err != nil {
		return err
	}
	paramRemovalMutex.Lock()
	globalParamRemovalList = parsed
	paramRemovalMutex.Unlock()
	return nil
}

func parseParamRemovalList(value string) ([]string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, nil
	}
	var raw []string
	if err := common.Unmarshal([]byte(trimmed), &raw); err != nil {
		return nil, fmt.Errorf("%s must be a JSON array of strings: %w", ParamRemovalListOptionKey, err)
	}
	normalized, err := dto.NormalizeParamRemovalList(raw)
	if err != nil {
		return nil, err
	}
	return normalized, nil
}

func GetParamRemovalBlacklistTTLSeconds() int {
	paramRemovalMutex.RLock()
	defer paramRemovalMutex.RUnlock()
	return paramRemovalBlacklistTTLSeconds
}

func GetParamRemovalBlacklistTTL() time.Duration {
	return time.Duration(GetParamRemovalBlacklistTTLSeconds()) * time.Second
}

func ValidateParamRemovalBlacklistTTL(value string) error {
	_, err := parseParamRemovalBlacklistTTL(value)
	return err
}

func UpdateParamRemovalBlacklistTTLFromString(value string) error {
	parsed, err := parseParamRemovalBlacklistTTL(value)
	if err != nil {
		return err
	}
	paramRemovalMutex.Lock()
	paramRemovalBlacklistTTLSeconds = parsed
	paramRemovalMutex.Unlock()
	return nil
}

func parseParamRemovalBlacklistTTL(value string) (int, error) {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer: %w", ParamRemovalBlacklistTTLSecondsOptionKey, err)
	}
	if parsed < minParamRemovalBlacklistTTLSeconds || parsed > maxParamRemovalBlacklistTTLSeconds {
		return 0, fmt.Errorf(
			"%s must be between %d and %d",
			ParamRemovalBlacklistTTLSecondsOptionKey,
			minParamRemovalBlacklistTTLSeconds,
			maxParamRemovalBlacklistTTLSeconds,
		)
	}
	return parsed, nil
}
