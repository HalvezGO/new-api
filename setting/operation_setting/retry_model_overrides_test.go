package operation_setting

import (
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateRetryModelOverridesFromJSONString(t *testing.T) {
	t.Cleanup(func() {
		require.NoError(t, UpdateRetryModelOverridesFromJSONString("{}"))
	})

	t.Run("parses overrides and trims the model name", func(t *testing.T) {
		require.NoError(t, UpdateRetryModelOverridesFromJSONString(`{" gpt-4o ": {"retry_times": 2, "retry_interval": 500}}`))

		override, ok := GetRetryModelOverride("gpt-4o")
		require.True(t, ok)
		require.NotNil(t, override.RetryTimes)
		assert.Equal(t, 2, *override.RetryTimes)
		require.NotNil(t, override.RetryInterval)
		assert.Equal(t, 500, *override.RetryInterval)
	})

	t.Run("an empty value clears the overrides", func(t *testing.T) {
		require.NoError(t, UpdateRetryModelOverridesFromJSONString(`{"gpt-4o": {"retry_times": 2}}`))
		require.NoError(t, UpdateRetryModelOverridesFromJSONString(""))

		_, ok := GetRetryModelOverride("gpt-4o")
		assert.False(t, ok)
	})

	t.Run("a rejected update keeps the previous overrides", func(t *testing.T) {
		require.NoError(t, UpdateRetryModelOverridesFromJSONString(`{"gpt-4o": {"retry_times": 2}}`))

		require.Error(t, UpdateRetryModelOverridesFromJSONString(
			fmt.Sprintf(`{"gpt-4o": {"retry_times": %d}}`, common.MaxRetryTimes+1),
		))

		override, ok := GetRetryModelOverride("gpt-4o")
		require.True(t, ok)
		require.NotNil(t, override.RetryTimes)
		assert.Equal(t, 2, *override.RetryTimes)
	})
}

func TestValidateRetryModelOverrides(t *testing.T) {
	valid := []string{
		"{}",
		"",
		`{"gpt-4o": {"retry_times": 0}}`,
		`{"gpt-4o": {"retry_interval": 0}}`,
		`{"gpt-4o": {"retry_times": 100, "retry_interval": 30000}}`,
	}
	for _, value := range valid {
		assert.NoError(t, ValidateRetryModelOverrides(value), value)
	}

	invalid := []string{
		"[]",
		"{",
		`{"": {"retry_times": 1}}`,
		`{"   ": {"retry_times": 1}}`,
		`{"gpt-4o": {}}`,
		`{"gpt-4o": {"retry_times": -1}}`,
		fmt.Sprintf(`{"gpt-4o": {"retry_times": %d}}`, common.MaxRetryTimes+1),
		`{"gpt-4o": {"retry_interval": -1}}`,
		fmt.Sprintf(`{"gpt-4o": {"retry_interval": %d}}`, common.MaxRetryInterval+1),
	}
	for _, value := range invalid {
		assert.Error(t, ValidateRetryModelOverrides(value), value)
	}
}
