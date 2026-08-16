package helper

import (
	"testing"

	"github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModelMappedHelper_WithModelPrefix(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(nil)
	c.Set("model_prefix", "byok-nvidia-nim/")
	c.Set("model_mapping", "")

	info := &common.RelayInfo{
		OriginModelName: "byok-nvidia-nim/openai/gpt-4o",
		ChannelMeta:     &common.ChannelMeta{},
	}
	request := &dto.GeneralOpenAIRequest{Model: "byok-nvidia-nim/openai/gpt-4o"}

	err := ModelMappedHelper(c, info, request)
	require.NoError(t, err)
	assert.Equal(t, "openai/gpt-4o", info.OriginModelName)
	assert.Equal(t, "openai/gpt-4o", info.UpstreamModelName)
	assert.Equal(t, "openai/gpt-4o", request.Model)
	assert.False(t, info.IsModelMapped)
}

func TestModelMappedHelper_WithModelPrefixAndMapping(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(nil)
	c.Set("model_prefix", "byok/")
	c.Set("model_mapping", `{"openai/gpt-4o": "openai/gpt-4"}`)

	info := &common.RelayInfo{
		OriginModelName: "byok/openai/gpt-4o",
		ChannelMeta:     &common.ChannelMeta{},
	}
	request := &dto.GeneralOpenAIRequest{Model: "byok/openai/gpt-4o"}

	err := ModelMappedHelper(c, info, request)
	require.NoError(t, err)
	// Prefix stripped first, then mapping applied
	assert.Equal(t, "openai/gpt-4", info.UpstreamModelName)
	assert.True(t, info.IsModelMapped)
}

func TestModelMappedHelper_WithEmptyPrefix(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(nil)
	c.Set("model_prefix", "")

	info := &common.RelayInfo{
		OriginModelName: "openai/gpt-4o",
		ChannelMeta: &common.ChannelMeta{
			UpstreamModelName: "openai/gpt-4o",
		},
	}
	request := &dto.GeneralOpenAIRequest{Model: "openai/gpt-4o"}

	err := ModelMappedHelper(c, info, request)
	require.NoError(t, err)
	assert.Equal(t, "openai/gpt-4o", info.OriginModelName)
	assert.Equal(t, "openai/gpt-4o", info.UpstreamModelName)
}

func TestModelMappedHelper_WithPrefixNoMatch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(nil)
	c.Set("model_prefix", "other-prefix/")

	info := &common.RelayInfo{
		OriginModelName: "openai/gpt-4o",
		ChannelMeta: &common.ChannelMeta{
			UpstreamModelName: "openai/gpt-4o",
		},
	}
	request := &dto.GeneralOpenAIRequest{Model: "openai/gpt-4o"}

	err := ModelMappedHelper(c, info, request)
	require.NoError(t, err)
	// Prefix doesn't match, so model name stays unchanged
	assert.Equal(t, "openai/gpt-4o", info.OriginModelName)
	assert.Equal(t, "openai/gpt-4o", info.UpstreamModelName)
}
