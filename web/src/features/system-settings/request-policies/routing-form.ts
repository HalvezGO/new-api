import type { TFunction } from 'i18next'
import { z } from 'zod'

import { parseHttpStatusCodeRules } from '@/lib/http-status-code-rules'
import {
  parseParamNameList,
  serializeParamNameList,
  validateParamNameList,
} from '@/lib/param-removal'

import { validateRetryModelOverrides } from '../models/routing-reliability-schema'
import { defaultRequestPolicySettings } from './defaults'

function parseParamRemovalListOption(value: string | undefined): string[] {
  if (!value) return []
  try {
    const parsed: unknown = JSON.parse(value)
    if (!Array.isArray(parsed)) return []
    return parsed.filter((item): item is string => typeof item === 'string')
  } catch {
    return []
  }
}

export function createRoutingPolicySchema(t: TFunction) {
  return z.object({
    RetryTimes: z.number().int().min(0).max(100),
    RetryInterval: z.number().int().min(0).max(30000),
    RetryIntervalStatusCodes: z
      .string()
      .refine(
        (value) => parseHttpStatusCodeRules(value).ok,
        t('Invalid status code rules')
      ),
    RetryModelOverrides: z.string().superRefine((value, context) => {
      const error = validateRetryModelOverrides(value)
      if (error) {
        context.addIssue({
          code: 'custom',
          message: t(error.message, error.params),
        })
      }
    }),
    ParamRemovalList: z.string().superRefine((value, context) => {
      if (!validateParamNameList(value).ok) {
        context.addIssue({
          code: 'custom',
          message: t(
            'Remove parameters must be comma-separated field names (letters, numbers, _, ., -), up to 100 entries'
          ),
        })
      }
    }),
    ParamRemovalBlacklistTTLSeconds: z.number().int().min(60).max(86400),
    AutomaticRetryStatusCodes: z
      .string()
      .refine(
        (value) => parseHttpStatusCodeRules(value).ok,
        t('Invalid status code rules')
      ),
    channel_affinity_setting: z.object({
      enabled: z.boolean(),
      session_mode: z.enum(['', 'off', 'prefer', 'strict']),
      switch_on_success: z.boolean(),
      keep_on_channel_disabled: z.boolean(),
      max_entries: z.number().int().min(0),
      default_ttl_seconds: z.number().int().min(0),
      rules: z.string().superRefine((value, context) => {
        try {
          const rules: unknown = JSON.parse(value)
          if (!Array.isArray(rules)) {
            context.addIssue({
              code: 'custom',
              message: t('Rules JSON must be an array'),
            })
          }
        } catch {
          context.addIssue({
            code: 'custom',
            message: t('Invalid rules JSON format'),
          })
        }
      }),
    }),
  })
}

export type RoutingPolicyFormValues = z.infer<
  ReturnType<typeof createRoutingPolicySchema>
>

export function routingPolicyFormValues(
  options: Record<string, string>
): RoutingPolicyFormValues {
  return {
    RetryTimes: Number(options.RetryTimes ?? 0),
    RetryInterval: Number(options.RetryInterval ?? 0),
    RetryIntervalStatusCodes:
      options.RetryIntervalStatusCodes ??
      defaultRequestPolicySettings.RetryIntervalStatusCodes,
    RetryModelOverrides:
      options.RetryModelOverrides ??
      defaultRequestPolicySettings.RetryModelOverrides,
    ParamRemovalList: serializeParamNameList(
      parseParamRemovalListOption(options.ParamRemovalList)
    ),
    ParamRemovalBlacklistTTLSeconds: Number(
      options.ParamRemovalBlacklistTTLSeconds ??
        defaultRequestPolicySettings.ParamRemovalBlacklistTTLSeconds
    ),
    AutomaticRetryStatusCodes:
      options.AutomaticRetryStatusCodes ??
      defaultRequestPolicySettings.AutomaticRetryStatusCodes,
    channel_affinity_setting: {
      enabled: options['channel_affinity_setting.enabled'] === 'true',
      session_mode: (options['channel_affinity_setting.session_mode'] ||
        '') as RoutingPolicyFormValues['channel_affinity_setting']['session_mode'],
      switch_on_success:
        options['channel_affinity_setting.switch_on_success'] === 'true',
      keep_on_channel_disabled:
        options['channel_affinity_setting.keep_on_channel_disabled'] === 'true',
      max_entries: Number(options['channel_affinity_setting.max_entries']),
      default_ttl_seconds: Number(
        options['channel_affinity_setting.default_ttl_seconds']
      ),
      rules: options['channel_affinity_setting.rules'] || '[]',
    },
  }
}

export function routingPolicyOptions(
  values: RoutingPolicyFormValues
): Record<string, string> {
  return {
    RetryTimes: String(values.RetryTimes),
    RetryInterval: String(values.RetryInterval),
    RetryIntervalStatusCodes: values.RetryIntervalStatusCodes,
    RetryModelOverrides: values.RetryModelOverrides.trim() || '{}',
    ParamRemovalList: JSON.stringify(
      parseParamNameList(values.ParamRemovalList)
    ),
    ParamRemovalBlacklistTTLSeconds: String(
      values.ParamRemovalBlacklistTTLSeconds
    ),
    AutomaticRetryStatusCodes: values.AutomaticRetryStatusCodes,
    ...Object.fromEntries(
      Object.entries(values.channel_affinity_setting).map(([key, value]) => [
        `channel_affinity_setting.${key}`,
        String(value),
      ])
    ),
  }
}
