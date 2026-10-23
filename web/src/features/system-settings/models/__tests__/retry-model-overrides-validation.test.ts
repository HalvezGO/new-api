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
import { describe, expect, test } from 'vitest'

import { createRoutingReliabilitySchema } from '../routing-reliability-schema'

const schema = createRoutingReliabilitySchema((key: string) => key)

const validValues = {
  RetryTimes: 0,
  RetryInterval: 0,
  RetryIntervalStatusCodes: '429',
  RetryModelOverrides: '{}',
  ChannelDisableThreshold: '',
  AutomaticDisableChannelEnabled: false,
  AutomaticEnableChannelEnabled: false,
  AutomaticDisableKeywords: '',
  AutomaticDisableStatusCodes: '',
  AutomaticRetryStatusCodes: '',
  monitor_setting: {
    auto_test_channel_enabled: false,
    auto_test_channel_minutes: 10,
    channel_test_concurrency: 1,
    channel_test_mode: 'scheduled_all' as const,
  },
}

function parseOverrides(value: string) {
  return schema.safeParse({ ...validValues, RetryModelOverrides: value })
}

describe('routing reliability per-model retry overrides', () => {
  test('accepts overrides that set either retry field', () => {
    const result = parseOverrides(
      JSON.stringify({
        'gpt-4o': { retry_times: 3, retry_interval: 2000 },
        'claude-3-5-sonnet': { retry_times: 0 },
      })
    )

    expect(result.success).toBe(true)
  })

  test('accepts an empty editor value as no overrides', () => {
    expect(parseOverrides('').success).toBe(true)
  })

  test('rejects malformed JSON', () => {
    expect(parseOverrides('{').success).toBe(false)
  })

  test('rejects documents that are not a JSON object', () => {
    expect(parseOverrides('[]').success).toBe(false)
  })

  test('rejects an empty model name', () => {
    expect(parseOverrides('{" ": {"retry_times": 1}}').success).toBe(false)
  })

  test('rejects an override without any retry field', () => {
    expect(parseOverrides('{"gpt-4o": {}}').success).toBe(false)
  })

  test('rejects unknown override fields', () => {
    expect(parseOverrides('{"gpt-4o": {"retry_time": 1}}').success).toBe(false)
  })

  test('rejects out-of-range or non-integer values', () => {
    expect(parseOverrides('{"gpt-4o": {"retry_times": 101}}').success).toBe(
      false
    )
    expect(parseOverrides('{"gpt-4o": {"retry_times": 1.5}}').success).toBe(
      false
    )
    expect(
      parseOverrides('{"gpt-4o": {"retry_interval": 30001}}').success
    ).toBe(false)
  })

  test('reports the issue on the RetryModelOverrides field', () => {
    const result = parseOverrides('{"gpt-4o": {"retry_times": 101}}')

    expect(result.success).toBe(false)
    if (result.success) return
    expect(result.error.issues[0]?.path).toEqual(['RetryModelOverrides'])
  })
})
