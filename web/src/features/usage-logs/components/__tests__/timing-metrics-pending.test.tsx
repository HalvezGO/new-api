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
import { act, render, screen } from '@testing-library/react'
import i18next from 'i18next'
import { afterEach, beforeAll, describe, expect, test, vi } from 'vitest'

import { TimingMetricsCell } from '../timing-metrics-cell'

describe('usage log in-progress timing display', () => {
  beforeAll(() => {
    i18next.addResourceBundle('en', 'translation', {
      'First token': 'First token',
      Duration: 'Duration',
      'N/A': 'N/A',
    })
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  test('shows the first-token latency as soon as it is known for a pending row', () => {
    render(
      <TimingMetricsCell
        useTimeSec={0}
        completionTokens={0}
        frtMs={1500}
        isStream
        isPending
      />
    )

    expect(screen.getByText('1.5s')).toBeInTheDocument()
  })

  test('keeps the pending duration counting from the row creation time', () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-10-20T10:00:00Z'))
    const createdAt = Math.floor(Date.now() / 1000) - 12

    render(
      <TimingMetricsCell
        useTimeSec={0}
        completionTokens={0}
        isStream
        isPending
        createdAt={createdAt}
      />
    )

    expect(screen.getByText('12.0s')).toBeInTheDocument()

    act(() => {
      vi.advanceTimersByTime(2000)
    })

    expect(screen.getByText('14.0s')).toBeInTheDocument()
  })

  test('keeps the settled values once the row completes', () => {
    render(
      <TimingMetricsCell
        useTimeSec={3.5}
        completionTokens={10}
        frtMs={1500}
        isStream
      />
    )

    expect(screen.getByText('1.5s')).toBeInTheDocument()
    expect(screen.getByText('3.5s')).toBeInTheDocument()
  })
})
