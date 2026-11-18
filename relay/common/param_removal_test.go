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
	t.Cleanup(func() {
		_ = operation_setting.UpdateParamRemovalListFromJSONString(`[]`)
	})
}

func paramNames(records []ParamRemovalRecord) []string {
	names := make([]string, 0, len(records))
	for _, record := range records {
		names = append(names, record.Param)
	}
	return names
}

func relayInfoWithChannelSetting(channelID int, model string, setting dto.ChannelSettings) *RelayInfo {
	return &RelayInfo{
		OriginModelName: model,
		ChannelMeta: &ChannelMeta{
			ChannelId:      channelID,
			ChannelSetting: setting,
		},
	}
}

func TestEffectiveParamRemovalRecordsPriority(t *testing.T) {
	resetParamRemovalSettings(t)
	require.NoError(t, operation_setting.UpdateParamRemovalListFromJSONString(`["global_param"]`))

	channelASetting := dto.ChannelSettings{
		RemoveParams: []string{"channel_param"},
		RetryModelOverrides: map[string]dto.RetryModelOverride{
			"model-a": {RemoveParams: []string{"model_param"}},
		},
	}
	channelBSetting := dto.ChannelSettings{RemoveParams: []string{"channel_param"}}

	infoA := relayInfoWithChannelSetting(1, "model-a", channelASetting)
	assert.Equal(t, []string{"model_param"}, paramNames(EffectiveParamRemovalRecords(infoA)))

	infoB := relayInfoWithChannelSetting(2, "model-a", channelBSetting)
	assert.Equal(t, []string{"channel_param"}, paramNames(EffectiveParamRemovalRecords(infoB)))

	otherModel := relayInfoWithChannelSetting(1, "model-b", channelASetting)
	assert.Equal(t, []string{"channel_param"}, paramNames(EffectiveParamRemovalRecords(otherModel)))

	globalOnly := relayInfoWithChannelSetting(1, "model-b", dto.ChannelSettings{})
	assert.Equal(t, []string{"global_param"}, paramNames(EffectiveParamRemovalRecords(globalOnly)))
}

func TestApplyParamRemovalStripsAndRecords(t *testing.T) {
	resetParamRemovalSettings(t)
	info := relayInfoWithChannelSetting(1, "model-a", dto.ChannelSettings{
		RemoveParams: []string{"prompt_cache_key", "temperature"},
	})
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
	info := relayInfoWithChannelSetting(1, "model-a", dto.ChannelSettings{})
	BlacklistUnsupportedParamsForRelayInfo(info, []string{"prompt_cache_key"}, time.Hour)

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

func TestApplyParamRemovalAutoBlacklistIsPerChannel(t *testing.T) {
	resetParamRemovalSettings(t)
	channelOne := relayInfoWithChannelSetting(1, "model-a", dto.ChannelSettings{})
	BlacklistUnsupportedParamsForRelayInfo(channelOne, []string{"prompt_cache_key"}, time.Hour)

	channelTwo := relayInfoWithChannelSetting(2, "model-a", dto.ChannelSettings{})

	out, err := ApplyParamRemoval(
		[]byte(`{"model":"model-a","prompt_cache_key":"cache"}`),
		channelTwo,
	)
	require.NoError(t, err)
	assert.JSONEq(t, `{"model":"model-a","prompt_cache_key":"cache"}`, string(out))
	assert.Empty(t, channelTwo.RemovedParams)
}

func TestApplyParamOverrideWithRelayInfoAppliesRemovalWithoutOverride(t *testing.T) {
	resetParamRemovalSettings(t)
	info := relayInfoWithChannelSetting(1, "model-a", dto.ChannelSettings{
		RemoveParams: []string{"prompt_cache_key"},
	})
	out, err := ApplyParamOverrideWithRelayInfo(
		[]byte(`{"model":"model-a","prompt_cache_key":"cache","messages":[]}`),
		info,
	)
	require.NoError(t, err)
	assert.JSONEq(t, `{"model":"model-a","messages":[]}`, string(out))
	require.Len(t, info.RemovedParams, 1)
	assert.Equal(t, "prompt_cache_key", info.RemovedParams[0].Param)
}
