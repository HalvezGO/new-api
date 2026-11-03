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

import type { RetryFailureEntry } from '../../types'
import { RetryHistoryDialog } from '../retry-history-dialog'

const failures: RetryFailureEntry[] = Array.from(
  { length: 12 },
  (_, index) => ({
    retry_index: index,
    channel_id: 12,
    channel_name: 'internal-channel',
    status_code: 429,
    error_code: 'rate_limit_exceeded',
    reason: `upstream rejected attempt ${index}`,
    waited_ms: 1500,
  })
)

describe('usage log retry history dialog', () => {
  beforeAll(() => {
    i18next.addResourceBundle('en', 'translation', {
      'Retry history': 'Retry history',
      'Retried {{count}} attempts': 'Retried {{count}} attempts',
      'Only the first {{count}} attempts are kept.':
        'Only the first {{count}} attempts are kept.',
      'Status Code': 'Status Code',
      'Error code': 'Error code',
      Channel: 'Channel',
      Waiting: 'Waiting',
      Reason: 'Reason',
      Close: 'Close',
      'No data': 'No data',
      'Total:': 'Total:',
      Page: 'Page',
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
    expect(dialog.getByText('1 / 2')).toBeVisible()

    await user.click(dialog.getByRole('button', { name: 'Go to next page' }))

    expect(
      await screen.findByText('upstream rejected attempt 10')
    ).toBeVisible()
    expect(screen.queryByText('upstream rejected attempt 0')).toBeNull()
    expect(screen.getByText('2 / 2')).toBeVisible()
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

  test('hides channel identity from non-admins but keeps failure reasons', async () => {
    const user = userEvent.setup()
    render(<RetryHistoryDialog failures={failures} isAdmin={false} />)

    await user.click(
      screen.getByRole('button', { name: /Retried 12 attempts/ })
    )

    const dialog = within(await screen.findByRole('dialog'))
    expect(dialog.queryByText(/internal-channel/)).toBeNull()
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
