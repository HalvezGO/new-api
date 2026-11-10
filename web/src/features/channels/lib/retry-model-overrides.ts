import {
  parseParamNameList,
  serializeParamNameList,
  validateParamNameList,
} from '@/lib/param-removal'

/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import {
  MAX_CHANNEL_RETRY_INTERVAL,
  MAX_CHANNEL_RETRY_TIMES,
} from './channel-form'

export type RetryModelOverride = {
  retry_times?: number
  retry_interval?: number
  remove_params?: string[]
}

export type RetryModelOverridesMap = Record<string, RetryModelOverride>

export type RetryModelOverrideRow = {
  id: string
  model: string
  retryTimes: string
  retryInterval: string
  removeParams: string
}

export type RetryOverrideValidation =
  | { ok: true }
  | { ok: false; message: string; params?: Record<string, unknown> }

type OptionalInteger = { valid: true; value: number | null } | { valid: false }

function parseOptionalInteger(value: string, max: number): OptionalInteger {
  const trimmed = value.trim()
  if (!trimmed) return { valid: true, value: null }
  if (!/^\d+$/.test(trimmed)) return { valid: false }
  const parsed = Number(trimmed)
  if (!Number.isInteger(parsed) || parsed > max) return { valid: false }
  return { valid: true, value: parsed }
}

function readOptionalInteger(value: unknown, max: number): number | undefined {
  if (typeof value !== 'number' || !Number.isInteger(value)) return undefined
  if (value < 0 || value > max) return undefined
  return value
}

export function parseRetryModelOverrides(
  value: string | null | undefined
): RetryModelOverridesMap {
  const trimmed = (value ?? '').trim()
  if (!trimmed) return {}

  let parsed: unknown
  try {
    parsed = JSON.parse(trimmed)
  } catch {
    return {}
  }
  if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) return {}

  const overrides: RetryModelOverridesMap = {}
  for (const [rawModel, rawOverride] of Object.entries(parsed)) {
    const model = rawModel.trim()
    if (
      !model ||
      !rawOverride ||
      typeof rawOverride !== 'object' ||
      Array.isArray(rawOverride)
    ) {
      continue
    }
    const source = rawOverride as Record<string, unknown>
    const retryTimes = readOptionalInteger(
      source.retry_times,
      MAX_CHANNEL_RETRY_TIMES
    )
    const retryInterval = readOptionalInteger(
      source.retry_interval,
      MAX_CHANNEL_RETRY_INTERVAL
    )
    const rawRemoveParams = Array.isArray(source.remove_params)
      ? source.remove_params.filter(
          (item): item is string => typeof item === 'string'
        )
      : []
    const removeParams = parseParamNameList(rawRemoveParams.join(','))
    if (
      retryTimes === undefined &&
      retryInterval === undefined &&
      removeParams.length === 0
    ) {
      continue
    }

    const override: RetryModelOverride = {}
    if (retryTimes !== undefined) override.retry_times = retryTimes
    if (retryInterval !== undefined) override.retry_interval = retryInterval
    if (removeParams.length > 0) override.remove_params = removeParams
    overrides[model] = override
  }
  return overrides
}

export function serializeRetryModelOverrides(
  overrides: RetryModelOverridesMap
): string {
  const ordered: RetryModelOverridesMap = {}
  for (const model of Object.keys(overrides).sort()) {
    const override = overrides[model]
    const normalized: RetryModelOverride = {}
    if (override.retry_times !== undefined) {
      normalized.retry_times = override.retry_times
    }
    if (override.retry_interval !== undefined) {
      normalized.retry_interval = override.retry_interval
    }
    if (override.remove_params && override.remove_params.length > 0) {
      normalized.remove_params = parseParamNameList(
        override.remove_params.join(',')
      )
    }
    if (
      normalized.retry_times === undefined &&
      normalized.retry_interval === undefined &&
      (!normalized.remove_params || normalized.remove_params.length === 0)
    ) {
      continue
    }
    ordered[model] = normalized
  }
  return JSON.stringify(ordered)
}

export function buildRetryOverrideRows(
  overrides: RetryModelOverridesMap,
  models: string[],
  createId: () => string
): RetryModelOverrideRow[] {
  const modelSet = new Set(models)
  return Object.keys(overrides)
    .filter((model) => modelSet.has(model))
    .sort()
    .map((model) => {
      const override = overrides[model]
      return {
        id: createId(),
        model,
        retryTimes:
          override.retry_times !== undefined
            ? String(override.retry_times)
            : '',
        retryInterval:
          override.retry_interval !== undefined
            ? String(override.retry_interval)
            : '',
        removeParams: serializeParamNameList(override.remove_params ?? []),
      }
    })
}

export function filterRetryOverrideRowsForModels(
  rows: RetryModelOverrideRow[],
  models: string[]
): RetryModelOverrideRow[] {
  const modelSet = new Set(models)
  return rows.filter((row) => !row.model || modelSet.has(row.model))
}

export function validateRetryOverrideRows(
  rows: RetryModelOverrideRow[],
  models: string[]
): RetryOverrideValidation {
  const modelSet = new Set(models)
  const seen = new Set<string>()

  for (const row of rows) {
    const model = row.model.trim()
    if (!model) {
      return {
        ok: false,
        message: 'Select a model for each retry override row',
      }
    }
    if (!modelSet.has(model)) continue
    if (seen.has(model)) {
      return {
        ok: false,
        message: 'Duplicate model in retry overrides: {{model}}',
        params: { model },
      }
    }
    seen.add(model)

    if (!parseOptionalInteger(row.retryTimes, MAX_CHANNEL_RETRY_TIMES).valid) {
      return {
        ok: false,
        message:
          'Retry times for {{model}} must be an integer between 0 and 100',
        params: { model },
      }
    }
    if (
      !parseOptionalInteger(row.retryInterval, MAX_CHANNEL_RETRY_INTERVAL).valid
    ) {
      return {
        ok: false,
        message:
          'Retry interval for {{model}} must be an integer between 0 and 30000',
        params: { model },
      }
    }
    const removeParamsValidation = validateParamNameList(row.removeParams)
    if (!removeParamsValidation.ok) {
      return {
        ok: false,
        message:
          'Remove parameters for {{model}} must be comma-separated field names (letters, numbers, _, ., -), up to 100 entries',
        params: { model },
      }
    }
  }

  return { ok: true }
}

export function buildRetryModelOverridesUpdate(args: {
  base: RetryModelOverridesMap
  initialModels: string[]
  currentModels: string[]
  rows: RetryModelOverrideRow[]
}): RetryModelOverridesMap {
  const next: RetryModelOverridesMap = { ...args.base }
  const initialScope = new Set(args.initialModels)
  for (const model of Object.keys(next)) {
    if (initialScope.has(model)) {
      delete next[model]
    }
  }

  const currentModelSet = new Set(args.currentModels)
  for (const row of args.rows) {
    const model = row.model.trim()
    if (!model || !currentModelSet.has(model)) continue

    const retryTimes = parseOptionalInteger(
      row.retryTimes,
      MAX_CHANNEL_RETRY_TIMES
    )
    const retryInterval = parseOptionalInteger(
      row.retryInterval,
      MAX_CHANNEL_RETRY_INTERVAL
    )
    if (!retryTimes.valid || !retryInterval.valid) continue
    const removeParams = parseParamNameList(row.removeParams)

    const override: RetryModelOverride = {}
    if (retryTimes.value !== null) override.retry_times = retryTimes.value
    if (retryInterval.value !== null) {
      override.retry_interval = retryInterval.value
    }
    if (removeParams.length > 0) override.remove_params = removeParams
    if (
      override.retry_times === undefined &&
      override.retry_interval === undefined &&
      (!override.remove_params || override.remove_params.length === 0)
    ) {
      continue
    }
    next[model] = override
  }

  return next
}
