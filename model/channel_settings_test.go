package model

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChannelValidateSettingsRejectsInvalidHTTPTransport(t *testing.T) {
	tests := []struct {
		name    string
		setting dto.ChannelSettings
		wantErr string
	}{
		{
			name:    "auto with shards is valid",
			setting: dto.ChannelSettings{HTTPProtocol: "auto", HTTP2ConnectionShards: 4},
		},
		{
			name:    "http1 with shards greater than one rejected",
			setting: dto.ChannelSettings{HTTPProtocol: "http1", HTTP2ConnectionShards: 2},
			wantErr: "http2_connection_shards",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			channel := &Channel{}
			channel.SetSetting(tt.setting)
			err := channel.ValidateSettings()
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestNormalizeModelsWithPrefix(t *testing.T) {
	tests := []struct {
		name      string
		models    string
		prefix    string
		expected  string
	}{
		{
			name:     "no prefix leaves models unchanged",
			models:   "gpt-4o,claude-3-5-sonnet",
			prefix:   "",
			expected: "gpt-4o,claude-3-5-sonnet",
		},
		{
			name:     "prefix applied to unprefixed models",
			models:   "openai/gpt-4o,anthropic/claude-3-5-sonnet",
			prefix:   "byok-nvidia-nim/",
			expected: "byok-nvidia-nim/openai/gpt-4o,byok-nvidia-nim/anthropic/claude-3-5-sonnet",
		},
		{
			name:     "already prefixed models stay unchanged",
			models:   "byok/openai/gpt-4o,byok/openai/gpt-4.1",
			prefix:   "byok/",
			expected: "byok/openai/gpt-4o,byok/openai/gpt-4.1",
		},
		{
			name:     "mixed prefixed and unprefixed normalized",
			models:   "byok/openai/gpt-4o,deepseek-ai/deepseek-v3,  ",
			prefix:   "byok/",
			expected: "byok/openai/gpt-4o,byok/deepseek-ai/deepseek-v3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			channel := &Channel{
				Models: tt.models,
			}
			channel.SetOtherSettings(dto.ChannelOtherSettings{ModelPrefix: tt.prefix})
			channel.NormalizeModelsWithPrefix()
			assert.Equal(t, tt.expected, channel.Models)
		})
	}
}

func TestAdvancedCustomChannelRequiresModelListRouteOnlyWhenUpdateChecksEnabled(t *testing.T) {
	inferenceRoute := dto.AdvancedCustomRoute{
		IncomingPath: "/v1/chat/completions",
		UpstreamPath: "/v1/chat/completions",
		Converter:    "none",
	}

	tests := []struct {
		name          string
		checksEnabled bool
		routes        []dto.AdvancedCustomRoute
		wantErr       string
	}{
		{
			name:   "legacy channel without discovery route remains valid",
			routes: []dto.AdvancedCustomRoute{inferenceRoute},
		},
		{
			name:          "enabled checks require discovery route",
			checksEnabled: true,
			routes:        []dto.AdvancedCustomRoute{inferenceRoute},
			wantErr:       dto.AdvancedCustomModelListPath,
		},
		{
			name:          "enabled checks accept discovery route",
			checksEnabled: true,
			routes: []dto.AdvancedCustomRoute{
				inferenceRoute,
				{
					IncomingPath: dto.AdvancedCustomModelListPath,
					UpstreamPath: dto.AdvancedCustomModelListPath,
					Converter:    "none",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			channel := &Channel{Type: constant.ChannelTypeAdvancedCustom}
			channel.SetOtherSettings(dto.ChannelOtherSettings{
				UpstreamModelUpdateCheckEnabled: tt.checksEnabled,
				AdvancedCustom: &dto.AdvancedCustomConfig{
					Routes: tt.routes,
				},
			})

			err := channel.ValidateSettings()
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}
