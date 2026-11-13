package common

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/MoeForge/moego/ojson"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

// ParamRemovalRecord identifies one request field stripped before an upstream
// attempt. Source is "config" for configured lists and "auto" for the
// per-model runtime blacklist created from an upstream unsupported-parameter
// error.
type ParamRemovalRecord struct {
	Param     string `json:"param"`
	Source    string `json:"source"`
	ExpiresAt int64  `json:"expires_at,omitempty"`
}

const (
	paramRemovalSourceConfig = "config"
	paramRemovalSourceAuto   = "auto"
)

type paramBlacklist struct {
	mu      sync.Mutex
	entries map[string]map[string]time.Time
}

var unsupportedParamBlacklist = paramBlacklist{entries: make(map[string]map[string]time.Time)}

// BlacklistUnsupportedParam records one unsupported parameter for a model until
// the supplied TTL expires. The blacklist is keyed by model only, so one
// logical model uses the same sanitized request across channels.
func BlacklistUnsupportedParam(model, param string, ttl time.Duration) (ParamRemovalRecord, bool) {
	model = strings.TrimSpace(model)
	param = strings.TrimSpace(param)
	if model == "" || param == "" {
		return ParamRemovalRecord{}, false
	}
	if ttl <= 0 {
		ttl = time.Hour
	}
	expiresAt := time.Now().Add(ttl)
	unsupportedParamBlacklist.mu.Lock()
	params := unsupportedParamBlacklist.entries[model]
	if params == nil {
		params = make(map[string]time.Time)
		unsupportedParamBlacklist.entries[model] = params
	}
	params[param] = expiresAt
	unsupportedParamBlacklist.mu.Unlock()
	return ParamRemovalRecord{Param: param, Source: paramRemovalSourceAuto, ExpiresAt: expiresAt.Unix()}, true
}

// BlacklistUnsupportedParams records all detected parameters and returns the
// records for the caller's logs.
func BlacklistUnsupportedParams(model string, params []string, ttl time.Duration) []ParamRemovalRecord {
	if len(params) == 0 {
		return nil
	}
	records := make([]ParamRemovalRecord, 0, len(params))
	for _, param := range params {
		if record, ok := BlacklistUnsupportedParam(model, param, ttl); ok {
			records = append(records, record)
		}
	}
	return records
}

// BlacklistUnsupportedParamsForRelayInfo records detected parameters under the
// same model key used by configured list resolution.
func BlacklistUnsupportedParamsForRelayInfo(info *RelayInfo, params []string, ttl time.Duration) []ParamRemovalRecord {
	return BlacklistUnsupportedParams(modelKeyForParamRemoval(info), params, ttl)
}

func blacklistedParamRecords(model string, now time.Time) []ParamRemovalRecord {
	model = strings.TrimSpace(model)
	if model == "" {
		return nil
	}
	unsupportedParamBlacklist.mu.Lock()
	defer unsupportedParamBlacklist.mu.Unlock()
	params := unsupportedParamBlacklist.entries[model]
	if len(params) == 0 {
		return nil
	}
	records := make([]ParamRemovalRecord, 0, len(params))
	for param, expiresAt := range params {
		if !expiresAt.After(now) {
			delete(params, param)
			continue
		}
		records = append(records, ParamRemovalRecord{
			Param:     param,
			Source:    paramRemovalSourceAuto,
			ExpiresAt: expiresAt.Unix(),
		})
	}
	if len(params) == 0 {
		delete(unsupportedParamBlacklist.entries, model)
	}
	return records
}

func modelKeyForParamRemoval(info *RelayInfo) string {
	if info == nil {
		return ""
	}
	model := strings.TrimSpace(info.OriginModelName)
	if info.ChannelMeta != nil {
		if prefix := strings.TrimSpace(info.ChannelOtherSettings.ModelPrefix); prefix != "" {
			model = strings.TrimPrefix(model, prefix)
		}
	}
	return model
}

// EffectiveParamRemovalRecords resolves configured lists with priority
// model > channel > global and unions the per-model runtime blacklist.
func EffectiveParamRemovalRecords(info *RelayInfo) []ParamRemovalRecord {
	if info == nil {
		return nil
	}
	model := modelKeyForParamRemoval(info)
	configured := []string(nil)
	if override, ok := operation_setting.GetRetryModelOverride(model); ok && len(override.RemoveParams) > 0 {
		configured = override.RemoveParams
	} else if info.ChannelMeta != nil && len(info.ChannelSetting.RemoveParams) > 0 {
		configured = info.ChannelSetting.RemoveParams
	} else {
		configured = operation_setting.GetGlobalParamRemovalList()
	}

	recordsByParam := make(map[string]ParamRemovalRecord, len(configured))
	for _, param := range configured {
		param = strings.TrimSpace(param)
		if param == "" {
			continue
		}
		recordsByParam[param] = ParamRemovalRecord{Param: param, Source: paramRemovalSourceConfig}
	}
	for _, record := range blacklistedParamRecords(model, time.Now()) {
		if _, exists := recordsByParam[record.Param]; !exists {
			recordsByParam[record.Param] = record
		}
	}
	if len(recordsByParam) == 0 {
		return nil
	}
	records := make([]ParamRemovalRecord, 0, len(recordsByParam))
	for _, record := range recordsByParam {
		records = append(records, record)
	}
	sort.Slice(records, func(i, j int) bool { return records[i].Param < records[j].Param })
	return records
}

// ApplyParamRemoval strips configured and blacklisted request fields from a
// JSON request body and records each field actually removed on RelayInfo. It
// edits the JSON document in place so unrelated numbers and fields keep their
// original representation.
func ApplyParamRemoval(jsonData []byte, info *RelayInfo) ([]byte, error) {
	records := EffectiveParamRemovalRecords(info)
	if len(records) == 0 {
		return jsonData, nil
	}
	// Only object bodies have top-level parameters; arrays and scalar bodies
	// (or malformed JSON) pass through untouched.
	if !ojson.ValidBytes(jsonData) || !ojson.GetBytesNoCopy(jsonData, "@this").IsObject() {
		return jsonData, nil
	}
	doc := ojson.NewDoc(jsonData)
	removed := make([]ParamRemovalRecord, 0, len(records))
	for _, record := range records {
		if !doc.Get(record.Param).Exists() {
			continue
		}
		if err := doc.Delete(record.Param); err != nil {
			return nil, fmt.Errorf("remove request parameter %s: %w", record.Param, err)
		}
		removed = append(removed, record)
	}
	if len(removed) == 0 {
		return jsonData, nil
	}
	mergeRemovedParams(info, removed)
	return doc.Bytes(), nil
}

func mergeRemovedParams(info *RelayInfo, removed []ParamRemovalRecord) {
	if info == nil || len(removed) == 0 {
		return
	}
	index := make(map[string]int, len(info.RemovedParams))
	for i, record := range info.RemovedParams {
		index[record.Param] = i
	}
	for _, record := range removed {
		if i, exists := index[record.Param]; exists {
			if info.RemovedParams[i].Source != paramRemovalSourceAuto {
				continue
			}
			info.RemovedParams[i] = record
			continue
		}
		index[record.Param] = len(info.RemovedParams)
		info.RemovedParams = append(info.RemovedParams, record)
	}
}
