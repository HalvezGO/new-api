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

import { buildApiParams } from '../utils'

describe('usage log API params', () => {
  test('forwards the status filter to the logs API', () => {
    const params = buildApiParams({
      page: 1,
      pageSize: 20,
      searchParams: { status: 1 },
      isAdmin: true,
    })

    expect(params.status).toBe(1)
  })

  test('omits the status filter when the URL has none', () => {
    const params = buildApiParams({
      page: 1,
      pageSize: 20,
      searchParams: {},
      isAdmin: true,
    })

    expect('status' in params).toBe(false)
  })
})
