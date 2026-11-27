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
import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import i18next from 'i18next'
import { beforeAll, describe, expect, test } from 'vitest'

import { Dialog } from '@/components/dialog'
import { formatTimestampToDate } from '@/lib/format'

import type { RetryFailureEntry } from '../../types'
import { RetryHistoryDialog } from '../retry-history-dialog'

const failures: RetryFailureEntry[] = Array.from(
  { length: 12 },
  (_, index) => ({
    retry_index: index,
    channel_id: 12,
    channel_name: 'internal-channel',
    key_index: 0,
    key_masked: 'sk-1**********cdef',
    status_code: 429,
    error_code: 'rate_limit_exceeded',
    reason: `upstream rejected attempt ${index}`,
    waited_ms: 1500,
    attempted_at: 1_700_000_000 + index,
  })
)

describe('usage log retry history dialog', () => {
  beforeAll(() => {
    i18next.addResourceBundle('en', 'translation', {
      'Retry history': 'Retry history',
      'Retried {{count}} attempts': 'Retried {{count}} attempts',
      'Only the first {{count}} attempts are kept.':
        'Only the first {{count}} attempts are kept.',
      'Only failed attempts are listed.': 'Only failed attempts are listed.',
      'Status Code': 'Status Code',
      'Error code': 'Error code',
      Channel: 'Channel',
      'API Key': 'API Key',
      Time: 'Time',
      Waiting: 'Waiting',
      Reason: 'Reason',
      Close: 'Close',
      'No data': 'No data',
      'Total:': 'Total:',
      Page: 'Page',
      'Rows per page': 'Rows per page',
      'Go to first page': 'Go to first page',
      'Go to last page': 'Go to last page',
      'Go to page {{page}}': 'Go to page {{page}}',
      'Go to previous page': 'Go to previous page',
      'Go to next page': 'Go to next page',
    })
  })

  test('opens a paginated retry table from the details entry', async () => {
    const user = userEvent.setup()
    render(<RetryHistoryDialog failures={failures} isAdmin />)

    expect(
      screen.getByRole('button', { name: /Retried 12 attempts/ })
    ).toBeVisible()
    expect(screen.queryByText('Status Code')).not.toBeInTheDocument()

    await user.click(
      screen.getByRole('button', { name: /Retried 12 attempts/ })
    )

    const dialog = within(await screen.findByRole('dialog'))
    expect(dialog.getByText('Status Code')).toBeVisible()
    expect(dialog.getByText('upstream rejected attempt 0')).toBeVisible()
    expect(dialog.queryByText('upstream rejected attempt 10')).toBeNull()
    expect(dialog.getByRole('combobox')).toBeVisible()

    await user.click(dialog.getByRole('button', { name: 'Go to next page' }))

    expect(
      await screen.findByText('upstream rejected attempt 10')
    ).toBeVisible()
    expect(screen.queryByText('upstream rejected attempt 0')).toBeNull()
  })

  test('uses compact pagination on small screens', async () => {
    const originalMatchMedia = window.matchMedia
    Object.defineProperty(window, 'matchMedia', {
      configurable: true,
      writable: true,
      value: (query: string) =>
        ({
          matches: query.includes('max-width: 640px'),
          media: query,
          onchange: null,
          addListener: () => undefined,
          removeListener: () => undefined,
          addEventListener: () => undefined,
          removeEventListener: () => undefined,
          dispatchEvent: () => false,
        }) as MediaQueryList,
    })

    try {
      const user = userEvent.setup()
      render(<RetryHistoryDialog failures={failures} isAdmin />)

      await user.click(
        screen.getByRole('button', { name: /Retried 12 attempts/ })
      )

      const dialog = within(await screen.findByRole('dialog'))
      expect(dialog.queryByRole('combobox')).toBeNull()
      expect(dialog.getByText('1 / 2')).toBeVisible()
    } finally {
      Object.defineProperty(window, 'matchMedia', {
        configurable: true,
        writable: true,
        value: originalMatchMedia,
      })
    }
  })

  test('changes the number of rows per page', async () => {
    const user = userEvent.setup()
    render(<RetryHistoryDialog failures={failures} isAdmin />)

    await user.click(
      screen.getByRole('button', { name: /Retried 12 attempts/ })
    )

    const dialog = within(await screen.findByRole('dialog'))
    expect(dialog.queryByText('upstream rejected attempt 10')).toBeNull()

    await user.click(dialog.getByRole('combobox'))
    await user.click(await screen.findByRole('option', { name: '20' }))

    expect(
      await dialog.findByText('upstream rejected attempt 10')
    ).toBeVisible()
  })

  test('shows channel identity and the truncation note to admins', async () => {
    const user = userEvent.setup()
    render(<RetryHistoryDialog failures={failures} truncated isAdmin />)

    await user.click(
      screen.getByRole('button', { name: /Retried 12 attempts…/ })
    )

    const dialog = within(await screen.findByRole('dialog'))
    expect(
      dialog.getAllByText('#12 (internal-channel)').length
    ).toBeGreaterThan(0)
    expect(
      dialog.getByText('Only the first 12 attempts are kept.')
    ).toBeVisible()
  })

  test('shows the attempted API key and attempt time to admins', async () => {
    const user = userEvent.setup()
    render(<RetryHistoryDialog failures={failures.slice(0, 1)} isAdmin />)

    await user.click(screen.getByRole('button', { name: /Retried 1 attempts/ }))

    const dialog = within(await screen.findByRole('dialog'))
    expect(dialog.getByText('API Key')).toBeVisible()
    expect(dialog.getByText('#0 · sk-1**********cdef')).toBeVisible()
    expect(dialog.getByText('Time')).toBeVisible()
    expect(
      dialog.getByText(formatTimestampToDate(failures[0].attempted_at ?? 0))
    ).toBeVisible()
  })

  test('keeps the table in its own scroll area so the horizontal scrollbar stays above pagination', async () => {
    const user = userEvent.setup()
    render(<RetryHistoryDialog failures={failures} isAdmin />)

    await user.click(
      screen.getByRole('button', { name: /Retried 12 attempts/ })
    )

    const dialog = within(await screen.findByRole('dialog'))
    const region = dialog.getByRole('region', { name: 'Retry history' })
    expect(region.className).toContain('flex-1')
    const scrollArea = region.querySelector('.overflow-auto')
    expect(scrollArea).not.toBeNull()
    // The table must scroll inside the flex area that fills the dialog body,
    // so the horizontal scrollbar stays above pagination instead of hugging
    // the last row.
    expect(scrollArea?.className).toContain('flex-1')
    expect(scrollArea?.className).toContain('min-h-0')
    expect(dialog.getByRole('table').parentElement).toBe(scrollArea)
    expect(region).not.toContainElement(dialog.getByRole('combobox'))
  })

  test('hides channel identity from non-admins but keeps failure reasons', async () => {
    const user = userEvent.setup()
    render(<RetryHistoryDialog failures={failures} isAdmin={false} />)

    await user.click(
      screen.getByRole('button', { name: /Retried 12 attempts/ })
    )

    const dialog = within(await screen.findByRole('dialog'))
    expect(dialog.queryByText(/internal-channel/)).toBeNull()
    expect(dialog.queryByText('API Key')).toBeNull()
    expect(dialog.queryByText(/sk-1\*+cdef/)).toBeNull()
    expect(dialog.getByText('Time')).toBeVisible()
    expect(dialog.getByText('upstream rejected attempt 0')).toBeVisible()
  })

  test('opens inside the log details dialog without closing it', async () => {
    const user = userEvent.setup()
    render(
      <Dialog open title='Log details' onOpenChange={() => undefined}>
        <RetryHistoryDialog failures={failures} isAdmin />
      </Dialog>
    )

    await user.click(
      screen.getByRole('button', { name: /Retried 12 attempts/ })
    )

    expect(await screen.findByText('Status Code')).toBeVisible()
    expect(screen.getByText('Log details')).toBeVisible()
  })

  test('renders nothing when there are no failed attempts', () => {
    const { container } = render(<RetryHistoryDialog failures={[]} isAdmin />)

    expect(container).toBeEmptyDOMElement()
  })
})
