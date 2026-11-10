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
import * as z from 'zod'

import { parseHttpStatusCodeRules } from '@/lib/http-status-code-rules'
import { validateParamNameList } from '@/lib/param-removal'

const numericString = z.string().refine((value) => {
  const trimmed = value.trim()
  if (!trimmed) return true
  return !Number.isNaN(Number(trimmed)) && Number(trimmed) >= 0
}, 'Enter a non-negative number or leave empty')

const channelTestModes = [
  'scheduled_all',
  'auto_ban_only',
  'passive_recovery',
] as const
export type ChannelTestMode = (typeof channelTestModes)[number]
export const MAX_CHANNEL_TEST_CONCURRENCY = 32
const MAX_RETRY_TIMES = 100
const MAX_RETRY_INTERVAL = 30000

function isIntegerInRange(value: unknown, min: number, max: number): boolean {
  return (
    typeof value === 'number' &&
    Number.isInteger(value) &&
    value >= min &&
    value <= max
  )
}

type RetryModelOverridesError = {
  message: string
  params?: Record<string, unknown>
}

export function validateRetryModelOverrides(
  value: string
): RetryModelOverridesError | null {
  const trimmed = value.trim()
  if (!trimmed) return null

  let parsed: unknown
  try {
    parsed = JSON.parse(trimmed)
  } catch {
    return { message: 'Invalid JSON format' }
  }
  if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
    return { message: 'Retry overrides must be a JSON object' }
  }

  for (const [rawModelName, rawOverride] of Object.entries(parsed)) {
    const modelName = rawModelName.trim()
    if (!modelName) {
      return { message: 'Model name must not be empty' }
    }
    if (
      !rawOverride ||
      typeof rawOverride !== 'object' ||
      Array.isArray(rawOverride)
    ) {
      return {
        message: 'Retry override for {{model}} must be a JSON object',
        params: { model: modelName },
      }
    }
    const override = rawOverride as Record<string, unknown>
    const keys = Object.keys(override)
    if (keys.length === 0) {
      return {
        message:
          'Retry override for {{model}} must set retry_times, retry_interval, or remove_params',
        params: { model: modelName },
      }
    }
    const unknownKeys = keys.filter(
      (key) =>
        key !== 'retry_times' &&
        key !== 'retry_interval' &&
        key !== 'remove_params'
    )
    if (unknownKeys.length > 0) {
      return {
        message:
          'Retry override for {{model}} contains unknown fields: {{fields}}',
        params: { model: modelName, fields: unknownKeys.join(', ') },
      }
    }
    if (
      override.retry_times !== undefined &&
      !isIntegerInRange(override.retry_times, 0, MAX_RETRY_TIMES)
    ) {
      return {
        message:
          'retry_times for {{model}} must be an integer between 0 and 100',
        params: { model: modelName },
      }
    }
    if (
      override.retry_interval !== undefined &&
      !isIntegerInRange(override.retry_interval, 0, MAX_RETRY_INTERVAL)
    ) {
      return {
        message:
          'retry_interval for {{model}} must be an integer between 0 and 30000',
        params: { model: modelName },
      }
    }
    if (override.remove_params !== undefined) {
      const removeParams = override.remove_params
      if (
        !Array.isArray(removeParams) ||
        removeParams.some((item) => typeof item !== 'string')
      ) {
        return {
          message: 'remove_params for {{model}} must be an array of strings',
          params: { model: modelName },
        }
      }
      const validation = validateParamNameList(removeParams.join(','))
      if (!validation.ok) {
        return {
          message:
            'remove_params for {{model}} must be comma-separated field names (letters, numbers, _, ., -), up to 100 entries',
          params: { model: modelName },
        }
      }
    }
  }
  return null
}

export const createRoutingReliabilitySchema = (
  t: (key: string, options?: Record<string, unknown>) => string
) =>
  z
    .object({
      RetryTimes: z.coerce.number().min(0).max(100),
      RetryInterval: z.coerce.number().min(0).max(30000),
      RetryIntervalStatusCodes: z.string(),
      RetryModelOverrides: z.string(),
      ChannelDisableThreshold: numericString,
      AutomaticDisableChannelEnabled: z.boolean(),
      AutomaticEnableChannelEnabled: z.boolean(),
      AutomaticDisableKeywords: z.string(),
      AutomaticDisableStatusCodes: z.string(),
      AutomaticRetryStatusCodes: z.string(),
      monitor_setting: z.object({
        auto_test_channel_enabled: z.boolean(),
        auto_test_channel_minutes: z.coerce
          .number()
          .int()
          .min(1, t('Interval must be at least 1 minute')),
        channel_test_concurrency: z.coerce
          .number()
          .int(t('Enter a positive integer'))
          .min(1, t('Channel test concurrency must be between 1 and 32'))
          .max(
            MAX_CHANNEL_TEST_CONCURRENCY,
            t('Channel test concurrency must be between 1 and 32')
          ),
        channel_test_mode: z.enum(channelTestModes),
      }),
    })
    .superRefine((values, ctx) => {
      const disableParsed = parseHttpStatusCodeRules(
        values.AutomaticDisableStatusCodes
      )
      if (!disableParsed.ok) {
        ctx.addIssue({
          code: 'custom',
          path: ['AutomaticDisableStatusCodes'],
          message: t('Invalid status code rules: {{tokens}}', {
            tokens: disableParsed.invalidTokens.join(', '),
          }),
        })
      }

      const retryParsed = parseHttpStatusCodeRules(
        values.AutomaticRetryStatusCodes
      )
      if (!retryParsed.ok) {
        ctx.addIssue({
          code: 'custom',
          path: ['AutomaticRetryStatusCodes'],
          message: t('Invalid status code rules: {{tokens}}', {
            tokens: retryParsed.invalidTokens.join(', '),
          }),
        })
      }

      const retryIntervalParsed = parseHttpStatusCodeRules(
        values.RetryIntervalStatusCodes
      )
      if (!retryIntervalParsed.ok) {
        ctx.addIssue({
          code: 'custom',
          path: ['RetryIntervalStatusCodes'],
          message: t('Invalid status code rules: {{tokens}}', {
            tokens: retryIntervalParsed.invalidTokens.join(', '),
          }),
        })
      }

      const retryModelOverridesError = validateRetryModelOverrides(
        values.RetryModelOverrides
      )
      if (retryModelOverridesError) {
        ctx.addIssue({
          code: 'custom',
          path: ['RetryModelOverrides'],
          message: t(
            retryModelOverridesError.message,
            retryModelOverridesError.params
          ),
        })
      }
    })

export type RoutingReliabilitySchema = ReturnType<
  typeof createRoutingReliabilitySchema
>
export type RoutingReliabilityFormValues = z.output<RoutingReliabilitySchema>
export type RoutingReliabilityFormInput = z.input<RoutingReliabilitySchema>
