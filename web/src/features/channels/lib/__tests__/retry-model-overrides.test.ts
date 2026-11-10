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

import {
  buildRetryModelOverridesUpdate,
  buildRetryOverrideRows,
  filterRetryOverrideRowsForModels,
  parseRetryModelOverrides,
  serializeRetryModelOverrides,
  validateRetryOverrideRows,
  type RetryModelOverrideRow,
} from '../retry-model-overrides'

function createRow(
  overrides: Partial<RetryModelOverrideRow> = {}
): RetryModelOverrideRow {
  return {
    id: overrides.id ?? 'row',
    model: '',
    retryTimes: '',
    retryInterval: '',
    removeParams: '',
    ...overrides,
  }
}

describe('retry model overrides', () => {
  test('reads only valid per-model overrides', () => {
    const parsed = parseRetryModelOverrides(
      JSON.stringify({
        ' gpt-4o ': { retry_times: 3, retry_interval: 2000 },
        'gpt-4o-mini': { retry_times: 0 },
        invalid: { retry_times: 101 },
        empty: {},
      })
    )

    expect(parsed).toEqual({
      'gpt-4o': { retry_times: 3, retry_interval: 2000 },
      'gpt-4o-mini': { retry_times: 0 },
    })
  })

  test('returns an empty map for malformed overrides', () => {
    expect(parseRetryModelOverrides('not json')).toEqual({})
    expect(parseRetryModelOverrides('')).toEqual({})
    expect(parseRetryModelOverrides('[]')).toEqual({})
  })

  test('serializes model keys in a stable order', () => {
    expect(
      serializeRetryModelOverrides({
        'model-b': { retry_interval: 1500 },
        'model-a': { retry_times: 3 },
      })
    ).toBe('{"model-a":{"retry_times":3},"model-b":{"retry_interval":1500}}')
  })

  test('builds rows only for channel models with overrides', () => {
    let nextId = 0
    const rows = buildRetryOverrideRows(
      {
        'model-a': { retry_times: 3 },
        'model-b': { retry_interval: 1500 },
        'model-c': { retry_times: 1 },
      },
      ['model-b', 'model-a'],
      () => `row-${(nextId += 1)}`
    )

    expect(rows).toEqual([
      {
        id: 'row-1',
        model: 'model-a',
        retryTimes: '3',
        retryInterval: '',
        removeParams: '',
      },
      {
        id: 'row-2',
        model: 'model-b',
        retryTimes: '',
        retryInterval: '1500',
        removeParams: '',
      },
    ])
  })

  test('keeps overrides that belong to models outside the channel', () => {
    const next = buildRetryModelOverridesUpdate({
      base: {
        'model-a': { retry_times: 3 },
        'model-b': { retry_times: 5 },
        'other-model': { retry_interval: 800 },
      },
      initialModels: ['model-a', 'model-b'],
      currentModels: ['model-a', 'model-b'],
      rows: [
        createRow({ model: 'model-a', retryTimes: '1', retryInterval: '250' }),
        createRow({ model: 'model-b', retryTimes: '5' }),
      ],
    })

    expect(next).toEqual({
      'model-a': { retry_times: 1, retry_interval: 250 },
      'model-b': { retry_times: 5 },
      'other-model': { retry_interval: 800 },
    })
  })

  test('deletes the override when its row is removed', () => {
    const next = buildRetryModelOverridesUpdate({
      base: {
        'model-a': { retry_times: 3 },
        'model-b': { retry_times: 5 },
      },
      initialModels: ['model-a', 'model-b'],
      currentModels: ['model-a', 'model-b'],
      rows: [createRow({ model: 'model-a', retryTimes: '3' })],
    })

    expect(next).toEqual({ 'model-a': { retry_times: 3 } })
  })

  test('deletes the override when both fields are cleared', () => {
    const next = buildRetryModelOverridesUpdate({
      base: { 'model-a': { retry_times: 3, retry_interval: 1500 } },
      initialModels: ['model-a'],
      currentModels: ['model-a'],
      rows: [createRow({ model: 'model-a' })],
    })

    expect(next).toEqual({})
  })

  test('deletes overrides for models removed from the channel', () => {
    const next = buildRetryModelOverridesUpdate({
      base: {
        'model-a': { retry_times: 3 },
        'model-b': { retry_times: 5 },
      },
      initialModels: ['model-a', 'model-b'],
      currentModels: ['model-a'],
      rows: [
        createRow({ model: 'model-a', retryTimes: '3' }),
        createRow({ model: 'model-b', retryTimes: '5' }),
      ],
    })

    expect(next).toEqual({ 'model-a': { retry_times: 3 } })
  })

  test('adds overrides for models newly added to the channel', () => {
    const next = buildRetryModelOverridesUpdate({
      base: { 'other-model': { retry_interval: 800 } },
      initialModels: ['model-a'],
      currentModels: ['model-a', 'model-c'],
      rows: [
        createRow({ model: 'model-a', retryTimes: '2' }),
        createRow({ model: 'model-c', retryInterval: '400' }),
      ],
    })

    expect(next).toEqual({
      'model-a': { retry_times: 2 },
      'model-c': { retry_interval: 400 },
      'other-model': { retry_interval: 800 },
    })
  })

  test('hides rows for models removed from the channel', () => {
    const rows = [
      createRow({ id: 'a', model: 'model-a' }),
      createRow({ id: 'b', model: 'model-b' }),
      createRow({ id: 'empty' }),
    ]

    expect(filterRetryOverrideRowsForModels(rows, ['model-b'])).toEqual([
      rows[1],
      rows[2],
    ])
  })

  test('rejects rows without a model or with duplicate models', () => {
    expect(
      validateRetryOverrideRows([createRow({ retryTimes: '1' })], ['model-a'])
    ).toEqual({
      ok: false,
      message: 'Select a model for each retry override row',
    })

    expect(
      validateRetryOverrideRows(
        [
          createRow({ model: 'model-a', retryTimes: '1' }),
          createRow({ model: 'model-a', retryTimes: '2' }),
        ],
        ['model-a']
      )
    ).toEqual({
      ok: false,
      message: 'Duplicate model in retry overrides: {{model}}',
      params: { model: 'model-a' },
    })
  })

  test('rejects retry values outside the supported ranges', () => {
    expect(
      validateRetryOverrideRows(
        [createRow({ model: 'model-a', retryTimes: '101' })],
        ['model-a']
      )
    ).toEqual({
      ok: false,
      message: 'Retry times for {{model}} must be an integer between 0 and 100',
      params: { model: 'model-a' },
    })

    expect(
      validateRetryOverrideRows(
        [createRow({ model: 'model-a', retryInterval: '30001' })],
        ['model-a']
      )
    ).toEqual({
      ok: false,
      message:
        'Retry interval for {{model}} must be an integer between 0 and 30000',
      params: { model: 'model-a' },
    })
  })

  test('accepts rows that only set one field', () => {
    expect(
      validateRetryOverrideRows(
        [
          createRow({ model: 'model-a', retryTimes: '0' }),
          createRow({ model: 'model-b', retryInterval: '30000' }),
        ],
        ['model-a', 'model-b']
      )
    ).toEqual({ ok: true })
  })

  test('reads and serializes per-model remove_params', () => {
    const parsed = parseRetryModelOverrides(
      JSON.stringify({
        'model-a': { remove_params: ['prompt_cache_key'] },
      })
    )
    expect(parsed).toEqual({
      'model-a': { remove_params: ['prompt_cache_key'] },
    })
    expect(serializeRetryModelOverrides(parsed)).toBe(
      '{"model-a":{"remove_params":["prompt_cache_key"]}}'
    )
  })

  test('includes removeParams when building rows', () => {
    const rows = buildRetryOverrideRows(
      {
        'model-a': { remove_params: ['prompt_cache_key'] },
      },
      ['model-a'],
      () => 'row-1'
    )
    expect(rows).toEqual([
      {
        id: 'row-1',
        model: 'model-a',
        retryTimes: '',
        retryInterval: '',
        removeParams: 'prompt_cache_key',
      },
    ])
  })

  test('rejects invalid remove_params rows and keeps valid ones in updates', () => {
    expect(
      validateRetryOverrideRows(
        [createRow({ model: 'model-a', removeParams: 'bad name' })],
        ['model-a']
      )
    ).toEqual({
      ok: false,
      message:
        'Remove parameters for {{model}} must be comma-separated field names (letters, numbers, _, ., -), up to 100 entries',
      params: { model: 'model-a' },
    })

    const next = buildRetryModelOverridesUpdate({
      base: {},
      initialModels: ['model-a'],
      currentModels: ['model-a'],
      rows: [
        createRow({
          model: 'model-a',
          removeParams: 'prompt_cache_key, temperature',
        }),
      ],
    })
    expect(next).toEqual({
      'model-a': { remove_params: ['prompt_cache_key', 'temperature'] },
    })
  })
})
