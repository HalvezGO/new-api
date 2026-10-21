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
import {
  flexRender,
  getCoreRowModel,
  useReactTable,
} from '@tanstack/react-table'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, test, vi } from 'vitest'

import type { UsageLog } from '../../data/schema'
import { useCommonLogsColumns } from '../columns/common-logs-columns'
import { UsageLogsDetailsDialog } from '../dialogs/usage-logs-details-dialog'
import { UsageLogsProvider } from '../usage-logs-provider'

// The icon loader pulls in an ESM-only icon package that jsdom cannot resolve;
// the details dialog does not depend on its output.
vi.mock('@/lib/lobe-icon', () => ({
  getLobeIcon: () => null,
}))

function buildLog(overrides: Partial<UsageLog> = {}): UsageLog {
  return {
    id: 1,
    user_id: 1,
    created_at: 1700000000,
    type: 2,
    content: 'log content',
    username: 'tester',
    token_name: 'test-token',
    model_name: 'gpt-4o',
    quota: 0,
    prompt_tokens: 12,
    completion_tokens: 0,
    use_time: 3,
    is_stream: false,
    channel: 7,
    channel_name: 'test-channel',
    token_id: 1,
    group: 'default',
    ip: '',
    other: '{}',
    request_id: 'req_details_test',
    upstream_request_id: '',
    status: 2,
    ...overrides,
  }
}

/**
 * Renders the details cell of the first log row. Re-rendering this harness with
 * new log objects recreates the column renderers, exactly like the auto-refresh
 * does when the query resolves again.
 */
function DetailsCellHarness({ logs }: { logs: UsageLog[] }) {
  const columns = useCommonLogsColumns(false)
  const table = useReactTable({
    data: logs,
    columns,
    getCoreRowModel: getCoreRowModel(),
  })
  const row = table.getRowModel().rows[0]
  const cell = row
    ?.getVisibleCells()
    .find((item) => item.column.id === 'content')

  return (
    <div>
      {cell ? flexRender(cell.column.columnDef.cell, cell.getContext()) : null}
    </div>
  )
}

describe('usage log details dialog', () => {
  test('stays open when the log list auto-refreshes', async () => {
    const user = userEvent.setup()

    const renderPage = (logs: UsageLog[]) => (
      <UsageLogsProvider>
        <UsageLogsDetailsDialog />
        <DetailsCellHarness logs={logs} />
      </UsageLogsProvider>
    )

    const { rerender } = render(renderPage([buildLog()]))

    await user.click(screen.getByTitle('Click to view full details'))
    expect(await screen.findByText('Log Details')).toBeInTheDocument()
    expect(screen.getByText('req_details_test')).toBeInTheDocument()

    rerender(renderPage([buildLog()]))

    expect(screen.getByText('Log Details')).toBeInTheDocument()
    expect(screen.getByText('req_details_test')).toBeInTheDocument()
  })
})
