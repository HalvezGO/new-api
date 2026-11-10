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
  MAX_PARAM_REMOVAL_LIST_SIZE,
  parseParamNameList,
  serializeParamNameList,
  validateParamNameList,
} from '../param-removal'

describe('param removal list helpers', () => {
  test('parses comma and newline separated names without duplicates', () => {
    expect(
      parseParamNameList(' prompt_cache_key, temperature\nprompt_cache_key ')
    ).toEqual(['prompt_cache_key', 'temperature'])
  })

  test('serializes and reparses the same names', () => {
    const params = ['prompt_cache_key', 'prompt_cache_options']
    expect(parseParamNameList(serializeParamNameList(params))).toEqual(params)
  })

  test('rejects names with unsupported characters', () => {
    expect(validateParamNameList('prompt_cache_key, bad name')).toEqual({
      ok: false,
      invalid: ['bad name'],
      tooMany: false,
    })
  })

  test('rejects lists above the maximum size', () => {
    const tooMany = Array.from(
      { length: MAX_PARAM_REMOVAL_LIST_SIZE + 1 },
      (_, index) => `param_${index}`
    ).join(',')
    expect(validateParamNameList(tooMany).ok).toBe(false)
  })
})
