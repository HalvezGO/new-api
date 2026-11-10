package service

import (
	"errors"
	"testing"

	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractUnsupportedParams(t *testing.T) {
	err := types.NewOpenAIError(
		errors.New("Validation: Unsupported parameter(s): `prompt_cache_key`, `prompt_cache_options`"),
		types.ErrorCodeBadResponseStatusCode,
		400,
	)
	assert.Equal(t, []string{"prompt_cache_key", "prompt_cache_options"}, ExtractUnsupportedParams(err))
}

func TestExtractUnsupportedParamsIgnoresNonClientErrors(t *testing.T) {
	err := types.NewOpenAIError(
		errors.New("Unsupported parameter(s): `prompt_cache_key`"),
		types.ErrorCodeBadResponseStatusCode,
		500,
	)
	assert.Nil(t, ExtractUnsupportedParams(err))
}

func TestAppendParamRemovalInfoExposesUserAndAdminDetails(t *testing.T) {
	other := model.NewLogOther()
	info := &relaycommon.RelayInfo{
		ParamRemovalRetryUsed: true,
		RemovedParams: []relaycommon.ParamRemovalRecord{
			{Param: "prompt_cache_key", Source: "auto", ExpiresAt: 1700000000},
			{Param: "temperature", Source: "config"},
		},
	}

	AppendParamRemovalInfo(other, info)
	snapshot := other.Snapshot()

	assert.Equal(t, []string{"prompt_cache_key", "temperature"}, snapshot["removed_params"])
	adminInfo, ok := snapshot["admin_info"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, true, adminInfo["param_removal_retry"])
	records, ok := adminInfo["param_removal"].([]map[string]any)
	require.True(t, ok)
	require.Len(t, records, 2)
	assert.Equal(t, "auto", records[0]["source"])
	assert.Equal(t, int64(1700000000), records[0]["expires_at"])
	assert.Equal(t, "config", records[1]["source"])
}
