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
import { render, screen } from '@testing-library/react'
import i18next from 'i18next'
import { beforeAll, describe, expect, test } from 'vitest'

import { StreamTpsCell } from '../timing-metrics-cell'

describe('usage log stream TPS display', () => {
  beforeAll(() => {
    i18next.addResourceBundle('en', 'translation', {
      Stream: 'Stream',
      'Non-stream': 'Non-stream',
    })
  })

  test('appends the post-first-token TPS in parentheses after the overall TPS', () => {
    render(
      <StreamTpsCell
        isStream
        tokensPerSecond={8}
        tokensPerSecondAfterFirstToken={222}
      />
    )

    expect(screen.getByText('8 t/s (222 t/s)')).toBeInTheDocument()
  })

  test('shows only the overall TPS when the first-token time is unknown', () => {
    render(<StreamTpsCell isStream tokensPerSecond={8} />)

    expect(screen.getByText('8 t/s')).toBeInTheDocument()
  })

  test('keeps the placeholder when no throughput is available', () => {
    render(<StreamTpsCell isStream tokensPerSecond={null} />)

    expect(screen.getByText('—')).toBeInTheDocument()
  })

  test('shows the in-progress state instead of throughput for a pending row', () => {
    i18next.addResourceBundle('en', 'translation', {
      'In progress': 'In progress',
    })
    render(<StreamTpsCell isStream isPending />)

    expect(screen.getByText('In progress')).toBeInTheDocument()
    expect(screen.queryByText(/t\/s/)).not.toBeInTheDocument()
  })

  test('shows the failure state instead of throughput for a failed row', () => {
    i18next.addResourceBundle('en', 'translation', {
      Failed: 'Failed',
    })
    render(<StreamTpsCell isStream status={2} />)

    expect(screen.getByText('Failed')).toBeInTheDocument()
    expect(screen.queryByText(/t\/s/)).not.toBeInTheDocument()
  })
})
