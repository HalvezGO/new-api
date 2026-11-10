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

import { channelSchema, type Channel } from '../../types'
import {
  buildSettingJSON,
  transformChannelToFormDefaults,
} from '../channel-form'

function buildChannel(setting: Record<string, unknown>): Channel {
  return channelSchema.parse({
    id: 1,
    type: 1,
    key: '',
    status: 1,
    name: 'test-channel',
    created_time: 0,
    test_time: 0,
    response_time: 0,
    balance_updated_time: 0,
    models: 'gpt-4o',
    setting: JSON.stringify(setting),
    settings: '{}',
  })
}

describe('channel retry settings', () => {
  test('reads stored retry overrides into the form', () => {
    const values = transformChannelToFormDefaults(
      buildChannel({ retry_times: 3, retry_interval: 1500 })
    )

    expect(values.retry_times).toBe('3')
    expect(values.retry_interval).toBe('1500')
  })

  test('leaves the retry fields empty when the channel inherits', () => {
    const values = transformChannelToFormDefaults(buildChannel({}))

    expect(values.retry_times).toBe('')
    expect(values.retry_interval).toBe('')
  })

  test('writes configured retry overrides as numbers', () => {
    const values = transformChannelToFormDefaults(
      buildChannel({ retry_times: 3, retry_interval: 1500 })
    )

    const setting = JSON.parse(buildSettingJSON(values))
    expect(setting.retry_times).toBe(3)
    expect(setting.retry_interval).toBe(1500)
  })

  test('omits retry overrides left empty so the channel inherits', () => {
    const values = transformChannelToFormDefaults(buildChannel({}))

    const setting = JSON.parse(buildSettingJSON(values))
    expect(setting).not.toHaveProperty('retry_times')
    expect(setting).not.toHaveProperty('retry_interval')
  })

  test('reads and writes the channel parameter removal list', () => {
    const values = transformChannelToFormDefaults(
      buildChannel({ remove_params: ['prompt_cache_key', 'temperature'] })
    )
    expect(values.remove_params).toBe('prompt_cache_key, temperature')

    const setting = JSON.parse(buildSettingJSON(values))
    expect(setting.remove_params).toEqual(['prompt_cache_key', 'temperature'])
  })

  test('omits an empty parameter removal list so the channel inherits', () => {
    const values = transformChannelToFormDefaults(buildChannel({}))
    const setting = JSON.parse(buildSettingJSON(values))
    expect(setting).not.toHaveProperty('remove_params')
  })
})
