package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"

	"github.com/stretchr/testify/require"
)

// TestFormatUserLogsStripsQuotaSaturation verifies the admin-only quota
// saturation marker (nested under other.admin_info) is removed for non-admin
// log views, since formatUserLogs strips the whole admin_info object.
func TestFormatUserLogsStripsQuotaSaturation(t *testing.T) {
	other := common.MapToJsonStr(map[string]interface{}{
		"model_price": 0.004,
		"admin_info": map[string]interface{}{
			"quota_saturation": map[string]interface{}{
				"op":      "QuotaFromDecimal",
				"kind":    "overflow",
				"clamped": common.MaxQuota,
			},
		},
	})
	logs := []*Log{{Other: other}}

	formatUserLogs(logs, 0)

	parsed, err := common.StrToMap(logs[0].Other)
	require.NoError(t, err)
	_, hasAdminInfo := parsed["admin_info"]
	require.False(t, hasAdminInfo, "admin_info (and nested quota_saturation) must be stripped for non-admin views")
	// Non-admin billing fields remain visible.
	require.Contains(t, parsed, "model_price")
}

// TestFormatUserLogsStripsRetryFailureChannelDetails verifies user-facing
// payloads never carry the channel identity of individual retry attempts: the
// channel column is admin-only and ChannelName is blanked for these views, so
// the retry history must not smuggle the same information back in.
func TestFormatUserLogsStripsRetryFailureChannelDetails(t *testing.T) {
	other := common.MapToJsonStr(map[string]interface{}{
		"failure_reason": "too many requests",
		"retry_failures": []map[string]interface{}{
			{
				"retry_index":  1,
				"channel_id":   12,
				"channel_name": "internal-channel-name",
				"status_code":  429,
				"reason":       "upstream 429",
			},
		},
	})
	logs := []*Log{{Other: other}}

	formatUserLogs(logs, 0)

	parsed, err := common.StrToMap(logs[0].Other)
	require.NoError(t, err)
	failures, ok := parsed["retry_failures"].([]interface{})
	require.True(t, ok, "retry_failures stays visible to the user")
	require.Len(t, failures, 1)
	entry, ok := failures[0].(map[string]interface{})
	require.True(t, ok)
	_, hasName := entry["channel_name"]
	_, hasId := entry["channel_id"]
	require.False(t, hasName, "retry attempt channel_name must be stripped for non-admin views")
	require.False(t, hasId, "retry attempt channel_id must be stripped for non-admin views")
	// The reason for each failed attempt stays visible: it is the user's own
	// request failure, and the API already returns that error to them.
	require.Contains(t, entry, "reason")
	require.Contains(t, entry, "status_code")
	require.Contains(t, parsed, "failure_reason")
}
