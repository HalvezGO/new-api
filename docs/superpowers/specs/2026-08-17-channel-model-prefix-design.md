# Channel Model Prefix Design

**Date:** 2026-08-17  
**Status:** Draft  
**Summary:** Add a `model_prefix` setting to channels so that model names with a configured prefix are automatically stripped when forwarding to upstream providers, eliminating the need for manual model_mapping entries.

## Problem

Users with BYOK (Bring Your Own Key) setups often prefix model names to distinguish channels (e.g., `byok-nvidia-nim/openai/gpt-oss-120b`). Currently, they must manually configure `model_mapping` entries to strip the prefix when forwarding to upstream providers. This is tedious when managing many models.

## Solution

Add an optional `model_prefix` field to channel settings. When configured:
- Client requests use prefixed model names (e.g., `byok-nvidia-nim/openai/gpt-oss-120b`)
- The system automatically strips the prefix before forwarding to upstream
- Existing `model_mapping` continues to work independently (prefix is stripped first, then mapping is applied)

## Architecture

### Data Flow

```
Client Request: model = "byok-nvidia-nim/openai/gpt-oss-120b"
    ↓
Ability lookup: matches prefixed model name ✓
    ↓
Distributor: sets context with model_prefix = "byok-nvidia-nim/"
    ↓
ModelMappedHelper: strips prefix → OriginModelName = "openai/gpt-oss-120b"
    ↓
Upstream Request: model = "openai/gpt-oss-120b"
```

### `/v1/models` Behavior (unchanged)

- Returns prefixed model names from `Ability` table
- No changes needed - clients already expect prefixed names

### Key Files to Modify

| File | Change |
|------|--------|
| `relaykit/dto/channel_settings.go` | Add `ModelPrefix` field |
| `relay/helper/model_mapped.go` | Strip prefix before mapping |
| `middleware/distributor.go` | Set prefix in context |
| `controller/channel_upstream_update.go` | Strip prefix in upstream model detection |
| `web/src/features/channels/lib/channel-form.ts` | Add form field |
| `web/src/features/channels/components/drawers/sections/channel-advanced-section.tsx` | Add UI |

## Implementation Details

### Backend Changes

#### 1. DTO (relaykit/dto/channel_settings.go)

```go
type ChannelOtherSettings struct {
    // ... existing fields ...
    ModelPrefix string `json:"model_prefix,omitempty"` // Model prefix to strip before upstream, e.g. "byok-nvidia-nim/"
}
```

#### 2. Model Mapped Helper (relay/helper/model_mapped.go)

In `ModelMappedHelper`, after getting `model_mapping` but before the mapping loop:

```go
// Strip model prefix if configured
modelPrefix := c.GetString("model_prefix")
if modelPrefix != "" && strings.HasPrefix(info.OriginModelName, modelPrefix) {
    info.OriginModelName = strings.TrimPrefix(info.OriginModelName, modelPrefix)
}
```

#### 3. Distributor (middleware/distributor.go)

After setting `model_mapping` context (line ~463):

```go
common.SetContextKey(c, constant.ContextKeyChannelModelMapping, channel.GetModelMapping())
modelPrefix := channel.GetOtherSettings().ModelPrefix
common.SetContextKey(c, "model_prefix", modelPrefix)
```

#### 4. Upstream Model Update (controller/channel_upstream_update.go)

In `collectPendingUpstreamModelChangesFromModels`, strip prefix when comparing:

```go
func collectPendingUpstreamModelChangesFromModels(
    localModels []string,
    upstreamModels []string,
    ignoredModels []string,
    modelMapping map[string]string,
    modelPrefix string,
) (pendingAddModels []string, pendingRemoveModels []string) {
    // Strip prefix from local models for comparison
    normalizedLocal := lo.Map(localModels, func(m string, _ int) string {
        if modelPrefix != "" && strings.HasPrefix(m, modelPrefix) {
            return strings.TrimPrefix(m, modelPrefix)
        }
        return m
    })
    // ... rest of function uses normalizedLocal
}
```

### Frontend Changes

#### 1. Form Schema (web/src/features/channels/lib/channel-form.ts)

```typescript
model_prefix: z
    .string()
    .max(100, t('channels.form.errors.modelPrefixTooLong'))
    .regex(/^[\w\-\/]+\/?$/, t('channels.form.errors.modelPrefixInvalid'))
    .optional()
    .nullable(),
```

Default value: `''`

#### 2. UI (channel-advanced-section.tsx or channel-mutate-drawer.tsx)

Add input field in the "Advanced Settings" or "Models & Groups" section:
- Label: "Model Prefix" / "模型前缀"
- Placeholder: e.g., "byok-nvidia-nim/"
- Description: "Prefix to strip from model names when forwarding to upstream"
- Validation: must end with `/` if non-empty, max 100 chars

#### 3. i18n

Add keys to `web/src/i18n/locales/en.json` and `zh.json`:
- `channels.form.modelPrefix` - "Model Prefix"
- `channels.form.modelPrefixDescription` - "Prefix to strip from model names when forwarding to upstream providers"
- `channels.form.errors.modelPrefixTooLong` - "Model prefix must be less than 100 characters"
- `channels.form.errors.modelPrefixInvalid` - "Model prefix must end with /"

## Behavior Rules

1. **Empty prefix**: No stripping, fully backward compatible
2. **Non-empty prefix**: Must end with `/` (validated)
3. **Prefix + model_mapping**: Prefix is stripped first, then model_mapping is applied
4. **Upstream model detection**: Strips prefix when comparing local vs upstream models
5. **`/v1/models`**: Returns prefixed names unchanged

## Testing

### Backend Tests

- `relay/helper/model_mapped_test.go`: Test prefix stripping with/without mapping
- `controller/channel_upstream_update_test.go`: Test model detection with prefix

### Frontend Tests

- Form validation tests for prefix field
- i18n key coverage

## Migration

No database migration needed - `model_prefix` is stored in existing `settings` JSON column.

## Rollback

Setting `model_prefix` to empty disables the feature for that channel.
