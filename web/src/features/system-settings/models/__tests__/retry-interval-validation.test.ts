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
    channel_test_mode: 'scheduled_all',
  },
}

describe('routing reliability retry interval validation', () => {
  test('accepts retry times up to 100 and interval up to 30000 ms', () => {
    const result = schema.safeParse({
      ...validValues,
      RetryTimes: 100,
      RetryInterval: 30000,
    })
    expect(result.success).toBe(true)
  })

  test('rejects out-of-range retry times and retry interval', () => {
    expect(schema.safeParse({ ...validValues, RetryTimes: 101 }).success).toBe(
      false
    )
    expect(
      schema.safeParse({ ...validValues, RetryInterval: 30001 }).success
    ).toBe(false)
    expect(
      schema.safeParse({ ...validValues, RetryInterval: -1 }).success
    ).toBe(false)
  })

  test('accepts comma-separated codes and inclusive ranges for retry interval codes', () => {
    const result = schema.safeParse({
      ...validValues,
      RetryIntervalStatusCodes: '429,500-599',
    })
    expect(result.success).toBe(true)
  })

  test('reports invalid retry interval status codes on their own field', () => {
    const result = schema.safeParse({
      ...validValues,
      RetryIntervalStatusCodes: 'foo',
    })
    expect(result.success).toBe(false)
    if (result.success) return
    expect(result.error.issues[0]?.path).toEqual(['RetryIntervalStatusCodes'])
  })
})
