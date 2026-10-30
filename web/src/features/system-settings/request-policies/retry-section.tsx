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
import { useFormContext } from 'react-hook-form'
import { useTranslation } from 'react-i18next'

import { JsonCodeEditor } from '@/components/json-code-editor'
import {
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { parseHttpStatusCodeRules } from '@/lib/http-status-code-rules'

import { SettingsCard } from '../components/settings-card'
import { safeNumberFieldProps } from '../utils/numeric-field'
import type { RoutingPolicyFormValues } from './routing-form'

const retryModelOverridesExample = JSON.stringify(
  {
    'gpt-4o': { retry_times: 3, retry_interval: 2000 },
    'claude-3-5-sonnet': { retry_times: 0 },
  },
  null,
  2
)

export function RetrySection() {
  const { t } = useTranslation()
  const form = useFormContext<RoutingPolicyFormValues>()
  const intervalStatusCodes = form.watch('RetryIntervalStatusCodes')
  const parsedIntervalStatusCodes =
    parseHttpStatusCodeRules(intervalStatusCodes)

  return (
    <SettingsCard title={t('Retry budget')} className='shadow-none'>
      <div className='space-y-4'>
        <FormField
          control={form.control}
          name='RetryTimes'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('Maximum retries')}</FormLabel>
              <FormControl>
                <Input
                  type='number'
                  min={0}
                  max={100}
                  step={1}
                  {...safeNumberFieldProps(field)}
                />
              </FormControl>
              <FormDescription>
                {t('Excludes the first attempt. Counted per group.')}
              </FormDescription>
              <FormMessage />
            </FormItem>
          )}
        />
        <FormField
          control={form.control}
          name='AutomaticRetryStatusCodes'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('Auto-retry status codes')}</FormLabel>
              <FormControl>
                <Input
                  {...field}
                  placeholder={t('e.g. 401, 403, 429, 500-599')}
                />
              </FormControl>
              <FormDescription>
                {t('2xx, 504 and 524 are always excluded.')}
              </FormDescription>
              <FormMessage />
            </FormItem>
          )}
        />
        <FormField
          control={form.control}
          name='RetryInterval'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('Retry Interval')}</FormLabel>
              <FormControl>
                <Input
                  type='number'
                  min={0}
                  max={30000}
                  step={1}
                  {...safeNumberFieldProps(field)}
                />
              </FormControl>
              <FormDescription>
                {t(
                  'Milliseconds to wait before the next retry when the previous failure matches the retry interval status codes. 0 means no wait.'
                )}
              </FormDescription>
              <FormMessage />
            </FormItem>
          )}
        />
        <FormField
          control={form.control}
          name='RetryIntervalStatusCodes'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('Retry interval status codes')}</FormLabel>
              <FormControl>
                <Input
                  value={field.value}
                  onChange={(event) => field.onChange(event.target.value)}
                  placeholder={t('e.g. 401, 403, 429, 500-599')}
                />
              </FormControl>
              <FormDescription>
                {t(
                  'Accepts comma-separated status codes and inclusive ranges.'
                )}{' '}
                {t(
                  'The retry interval is applied when the previous failure matches one of these codes.'
                )}{' '}
                {parsedIntervalStatusCodes.ok &&
                  parsedIntervalStatusCodes.normalized &&
                  parsedIntervalStatusCodes.normalized !==
                    field.value.trim() && (
                    <span className='text-muted-foreground'>
                      {t('Normalized:')} {parsedIntervalStatusCodes.normalized}
                    </span>
                  )}
              </FormDescription>
              <FormMessage />
            </FormItem>
          )}
        />
        <FormField
          control={form.control}
          name='RetryModelOverrides'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('Per-model retry overrides')}</FormLabel>
              <FormControl>
                <JsonCodeEditor
                  value={field.value}
                  onChange={(value) => field.onChange(value)}
                  name={field.name}
                  onBlur={field.onBlur}
                  textareaRef={field.ref}
                  placeholder={`${t('Example:')}\n${retryModelOverridesExample}`}
                  heightClassName='h-40 min-h-40 max-h-40'
                />
              </FormControl>
              <FormDescription>
                {t(
                  'Model keys match the model name requested by the client. Model overrides take priority over channel settings, which take priority over the global Retry Times and Retry Interval options.'
                )}
              </FormDescription>
              <FormMessage />
            </FormItem>
          )}
        />
      </div>
    </SettingsCard>
  )
}
