package common

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func resetParamRemovalSettings(t *testing.T) {
	t.Helper()
	require.NoError(t, operation_setting.UpdateParamRemovalListFromJSONString(`[]`))
	require.NoError(t, operation_setting.UpdateRetryModelOverridesFromJSONString(`{}`))
	t.Cleanup(func() {
		_ = operation_setting.UpdateParamRemovalListFromJSONString(`[]`)
		_ = operation_setting.UpdateRetryModelOverridesFromJSONString(`{}`)
	})
}

func paramNames(records []ParamRemovalRecord) []string {
	names := make([]string, 0, len(records))
	for _, record := range records {
		names = append(names, record.Param)
	}
	return names
}

func TestEffectiveParamRemovalRecordsPriority(t *testing.T) {
	resetParamRemovalSettings(t)
	require.NoError(t, operation_setting.UpdateParamRemovalListFromJSONString(`["global_param"]`))
	require.NoError(t, operation_setting.UpdateRetryModelOverridesFromJSONString(`{"model-a":{"remove_params":["model_param"]}}`))

	info := &RelayInfo{
		OriginModelName: "model-a",
		ChannelMeta:     &ChannelMeta{ChannelSetting: dto.ChannelSettings{RemoveParams: []string{"channel_param"}}},
	}
	assert.Equal(t, []string{"model_param"}, paramNames(EffectiveParamRemovalRecords(info)))

	info.OriginModelName = "model-b"
	assert.Equal(t, []string{"channel_param"}, paramNames(EffectiveParamRemovalRecords(info)))

	info.ChannelSetting.RemoveParams = nil
	assert.Equal(t, []string{"global_param"}, paramNames(EffectiveParamRemovalRecords(info)))
}

func TestApplyParamRemovalStripsAndRecords(t *testing.T) {
	resetParamRemovalSettings(t)
	info := &RelayInfo{
		OriginModelName: "model-a",
		ChannelMeta: &ChannelMeta{ChannelSetting: dto.ChannelSettings{
			RemoveParams: []string{"prompt_cache_key", "temperature"},
		}},
	}
	out, err := ApplyParamRemoval(
		[]byte(`{"model":"model-a","prompt_cache_key":"cache","seed":9007199254740993,"messages":[]}`),
		info,
	)
	require.NoError(t, err)
	assert.JSONEq(t, `{"model":"model-a","seed":9007199254740993,"messages":[]}`, string(out))
	assert.Contains(t, string(out), "9007199254740993")
	assert.Equal(t, []ParamRemovalRecord{
		{Param: "prompt_cache_key", Source: paramRemovalSourceConfig},
	}, info.RemovedParams)
}

func TestApplyParamRemovalUsesAutoBlacklist(t *testing.T) {
	resetParamRemovalSettings(t)
	BlacklistUnsupportedParam("model-a", "prompt_cache_key", time.Hour)
	info := &RelayInfo{OriginModelName: "model-a"}
	out, err := ApplyParamRemoval(
		[]byte(`{"model":"model-a","prompt_cache_key":"cache"}`),
		info,
	)
	require.NoError(t, err)
	assert.JSONEq(t, `{"model":"model-a"}`, string(out))
	require.Len(t, info.RemovedParams, 1)
	assert.Equal(t, "prompt_cache_key", info.RemovedParams[0].Param)
	assert.Equal(t, paramRemovalSourceAuto, info.RemovedParams[0].Source)
	assert.NotZero(t, info.RemovedParams[0].ExpiresAt)
}

func TestApplyParamOverrideWithRelayInfoAppliesRemovalWithoutOverride(t *testing.T) {
	resetParamRemovalSettings(t)
	info := &RelayInfo{
		OriginModelName: "model-a",
		ChannelMeta: &ChannelMeta{ChannelSetting: dto.ChannelSettings{
			RemoveParams: []string{"prompt_cache_key"},
		}},
	}
	out, err := ApplyParamOverrideWithRelayInfo(
		[]byte(`{"model":"model-a","prompt_cache_key":"cache","messages":[]}`),
		info,
	)
	require.NoError(t, err)
	assert.JSONEq(t, `{"model":"model-a","messages":[]}`, string(out))
	require.Len(t, info.RemovedParams, 1)
	assert.Equal(t, "prompt_cache_key", info.RemovedParams[0].Param)
}
