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
import { describe, expect, test } from 'vitest'

import type { UsageLog } from '../../data/schema'
import { DetailsDialog } from '../dialogs/details-dialog'

function buildLog(other: Record<string, unknown>, isStream = false): UsageLog {
  return {
    id: 1,
    user_id: 1,
    created_at: 1700000000,
    type: 2,
    content: '',
    username: 'tester',
    token_name: 'test-token',
    model_name: 'gpt-4o',
    quota: 0,
    prompt_tokens: 12,
    completion_tokens: 0,
    use_time: 3,
    is_stream: isStream,
    channel: 7,
    channel_name: 'test-channel',
    token_id: 1,
    group: 'default',
    ip: '',
    other: JSON.stringify(other),
    request_id: 'req_test',
    upstream_request_id: '',
    status: 2,
  }
}

function renderDialog(log: UsageLog) {
  return render(
    <DetailsDialog
      log={log}
      isAdmin={false}
      open
      onOpenChange={() => undefined}
    />
  )
}

describe('usage log details dialog failure reason', () => {
  test('shows a friendly label instead of the raw error when the client disconnected', () => {
    renderDialog(
      buildLog({
        client_gone: true,
        failure_reason: 'status_code=499, context canceled',
      })
    )

    expect(screen.getByText('Client disconnected')).toBeInTheDocument()
    expect(
      screen.queryByText('status_code=499, context canceled')
    ).not.toBeInTheDocument()
  })

  test('keeps showing the raw upstream failure reason otherwise', () => {
    renderDialog(
      buildLog({ failure_reason: 'status_code=503, upstream unavailable' })
    )

    expect(
      screen.getByText('status_code=503, upstream unavailable')
    ).toBeInTheDocument()
    expect(screen.queryByText('Client disconnected')).not.toBeInTheDocument()
  })

  test('marks a stream that ended because the client left the same way', () => {
    renderDialog(
      buildLog(
        {
          stream_status: { status: 'error', end_reason: 'client_gone' },
        },
        true
      )
    )

    expect(screen.getByText('Client disconnected')).toBeInTheDocument()
    expect(screen.getByText('Stream Status')).toBeInTheDocument()
  })
})
