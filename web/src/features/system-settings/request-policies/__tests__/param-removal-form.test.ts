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

import { routingPolicyFormValues, routingPolicyOptions } from '../routing-form'

describe('request policy parameter removal form', () => {
  test('parses and serializes the global removal list and TTL', () => {
    const values = routingPolicyFormValues({
      ParamRemovalList: '["prompt_cache_key","temperature"]',
      ParamRemovalBlacklistTTLSeconds: '120',
    })

    expect(values.ParamRemovalList).toBe('prompt_cache_key, temperature')
    expect(values.ParamRemovalBlacklistTTLSeconds).toBe(120)

    const options = routingPolicyOptions(values)
    expect(options.ParamRemovalList).toBe('["prompt_cache_key","temperature"]')
    expect(options.ParamRemovalBlacklistTTLSeconds).toBe('120')
  })
})
