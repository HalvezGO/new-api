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
  getTokensPerSecond,
  getTokensPerSecondAfterFirstToken,
} from '../format'

describe('usage log throughput', () => {
  test('computes overall TPS over the whole request duration', () => {
    expect(getTokensPerSecond(289, 35)).toBeCloseTo(8.2571, 3)
  })

  test('returns null when there is no positive duration or tokens', () => {
    expect(getTokensPerSecond(100, 0)).toBeNull()
    expect(getTokensPerSecond(0, 10)).toBeNull()
    expect(getTokensPerSecond(-1, 10)).toBeNull()
  })

  test('computes post-first-token TPS over the generation window', () => {
    // 289 tokens generated in 35s - 33.7s first-token wait = 1.3s window.
    expect(getTokensPerSecondAfterFirstToken(289, 35, 33700)).toBeCloseTo(
      222.3077,
      3
    )
  })

  test('omits the post-first-token TPS when the first-token time is unknown', () => {
    expect(getTokensPerSecondAfterFirstToken(289, 35, undefined)).toBeNull()
    expect(getTokensPerSecondAfterFirstToken(289, 35, null)).toBeNull()
    expect(getTokensPerSecondAfterFirstToken(289, 35, 0)).toBeNull()
    expect(getTokensPerSecondAfterFirstToken(289, 35, -1000)).toBeNull()
  })

  test('omits the post-first-token TPS when no positive generation window remains', () => {
    expect(getTokensPerSecondAfterFirstToken(289, 35, 35000)).toBeNull()
    expect(getTokensPerSecondAfterFirstToken(289, 35, 40000)).toBeNull()
    expect(getTokensPerSecondAfterFirstToken(0, 35, 33700)).toBeNull()
  })
})
