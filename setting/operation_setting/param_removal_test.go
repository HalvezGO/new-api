package operation_setting

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParamRemovalSettings(t *testing.T) {
	t.Cleanup(func() {
		_ = UpdateParamRemovalListFromJSONString("[]")
		_ = UpdateParamRemovalBlacklistTTLFromString("3600")
	})

	require.NoError(t, UpdateParamRemovalListFromJSONString(`["prompt_cache_key","temperature"]`))
	assert.Equal(t, []string{"prompt_cache_key", "temperature"}, GetGlobalParamRemovalList())
	assert.Equal(t, `["prompt_cache_key","temperature"]`, ParamRemovalListToJSONString())

	assert.Error(t, ValidateParamRemovalList(`{"bad":true}`))
	assert.Error(t, ValidateParamRemovalList(`["bad name"]`))

	require.NoError(t, UpdateParamRemovalBlacklistTTLFromString("120"))
	assert.Equal(t, 120, GetParamRemovalBlacklistTTLSeconds())
	assert.Equal(t, 2*time.Minute, GetParamRemovalBlacklistTTL())
	assert.Error(t, ValidateParamRemovalBlacklistTTL("10"))
}
