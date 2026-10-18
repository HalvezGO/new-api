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
  AUTO_REFRESH_DEFAULT_SECONDS,
  AUTO_REFRESH_MAX_SECONDS,
  AUTO_REFRESH_MIN_SECONDS,
  clampAutoRefreshSeconds,
} from '../auto-refresh'

describe('auto refresh interval', () => {
  test('keeps a value inside the allowed range', () => {
    expect(clampAutoRefreshSeconds(15)).toBe(15)
    expect(clampAutoRefreshSeconds('30')).toBe(30)
  })

  test('clamps values below the minimum up to the minimum', () => {
    expect(clampAutoRefreshSeconds(0)).toBe(AUTO_REFRESH_MIN_SECONDS)
    expect(clampAutoRefreshSeconds(-8)).toBe(AUTO_REFRESH_MIN_SECONDS)
  })

  test('clamps values above the maximum down to the maximum', () => {
    expect(clampAutoRefreshSeconds(3600)).toBe(AUTO_REFRESH_MAX_SECONDS)
  })

  test('falls back to the default for empty or non-numeric input', () => {
    expect(clampAutoRefreshSeconds('')).toBe(AUTO_REFRESH_DEFAULT_SECONDS)
    expect(clampAutoRefreshSeconds('abc')).toBe(AUTO_REFRESH_DEFAULT_SECONDS)
    expect(clampAutoRefreshSeconds(undefined)).toBe(
      AUTO_REFRESH_DEFAULT_SECONDS
    )
    expect(clampAutoRefreshSeconds(Number.NaN)).toBe(
      AUTO_REFRESH_DEFAULT_SECONDS
    )
  })

  test('truncates fractional input to whole seconds', () => {
    expect(clampAutoRefreshSeconds('7.9')).toBe(7)
  })
})
