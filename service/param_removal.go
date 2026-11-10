package service

import (
	"regexp"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

var (
	unsupportedParamMessagePattern = regexp.MustCompile(`(?is)unsupported\s+parameter(?:\(s\)|s)?\s*:\s*(.+)`)
	quotedParamPattern             = regexp.MustCompile("`([^`]+)`|\"([^\"]+)\"|'([^']+)'")
	bareParamPattern               = regexp.MustCompile(`[A-Za-z0-9_.-]{1,128}`)
)

var nonParamWords = map[string]struct{}{
	"and": {}, "are": {}, "field": {}, "fields": {}, "for": {}, "invalid": {},
	"is": {}, "not": {}, "parameter": {}, "parameters": {}, "request": {},
	"supported": {}, "the": {}, "unsupported": {}, "was": {}, "with": {},
}

// ExtractUnsupportedParams parses an upstream validation error for the
// parameters it says are unsupported. Only 4xx responses are considered.
func ExtractUnsupportedParams(err *types.NewAPIError) []string {
	if err == nil {
		return nil
	}
	if err.StatusCode < 400 || err.StatusCode >= 500 {
		return nil
	}
	text := err.Error()
	match := unsupportedParamMessagePattern.FindStringSubmatch(text)
	if len(match) < 2 {
		return nil
	}
	tail := match[1]
	names := make([]string, 0, 4)
	for _, quoted := range quotedParamPattern.FindAllStringSubmatch(tail, -1) {
		for i := 1; i < len(quoted); i++ {
			if quoted[i] != "" {
				names = append(names, quoted[i])
			}
		}
	}
	if len(names) == 0 {
		for _, candidate := range bareParamPattern.FindAllString(tail, -1) {
			if _, skip := nonParamWords[strings.ToLower(candidate)]; skip {
				continue
			}
			names = append(names, candidate)
		}
	}
	normalized, normalizeErr := dto.NormalizeParamRemovalList(names)
	if normalizeErr != nil {
		return nil
	}
	return normalized
}

// BlacklistUnsupportedParams records detected parameters for the one-hour
// per-model runtime blacklist (TTL configurable in the request policy).
func BlacklistUnsupportedParams(info *relaycommon.RelayInfo, params []string) []relaycommon.ParamRemovalRecord {
	if info == nil || len(params) == 0 {
		return nil
	}
	return relaycommon.BlacklistUnsupportedParamsForRelayInfo(
		info,
		params,
		operation_setting.GetParamRemovalBlacklistTTL(),
	)
}

// AppendParamRemovalInfo writes removed parameter names to the user-visible log
// payload and the full source/expiry details to the admin-only payload.
func AppendParamRemovalInfo(other *model.LogOther, info *relaycommon.RelayInfo) {
	if other == nil || info == nil {
		return
	}
	if info.ParamRemovalRetryUsed {
		other.SetAdmin("param_removal_retry", true)
	}
	if len(info.RemovedParams) == 0 {
		return
	}
	names := make([]string, 0, len(info.RemovedParams))
	adminRecords := make([]map[string]any, 0, len(info.RemovedParams))
	for _, record := range info.RemovedParams {
		names = append(names, record.Param)
		detail := map[string]any{
			"param":  record.Param,
			"source": record.Source,
		}
		if record.ExpiresAt > 0 {
			detail["expires_at"] = record.ExpiresAt
		}
		adminRecords = append(adminRecords, detail)
	}
	sort.Strings(names)
	other.SetPublic("removed_params", names)
	other.SetAdmin("param_removal", adminRecords)
}
