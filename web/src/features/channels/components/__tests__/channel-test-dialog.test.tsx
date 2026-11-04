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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useEffect } from 'react'
import { beforeEach, describe, expect, test, vi } from 'vitest'

import { updateChannel } from '../../api'
import { handleTestChannel } from '../../lib'
import { channelSchema, type Channel } from '../../types'
import { ChannelsProvider, useChannels } from '../channels-provider'
import { ChannelTestDialog } from '../dialogs/channel-test-dialog'

vi.mock('../../api', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../../api')>()
  return {
    ...actual,
    updateChannel: vi.fn(),
  }
})

vi.mock('../../lib', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../../lib')>()
  return {
    ...actual,
    handleTestChannel: vi.fn(),
  }
})

const channel: Channel = channelSchema.parse({
  id: 7,
  type: 1,
  key: 'test-key',
  status: 1,
  name: 'Test Channel',
  created_time: 0,
  test_time: 0,
  response_time: 0,
  balance_updated_time: 0,
  models: 'model-a,model-b,model-c',
})

function Harness(props: { channel: Channel }) {
  const { setCurrentRow } = useChannels()

  useEffect(() => {
    setCurrentRow(props.channel)
  }, [props.channel, setCurrentRow])

  return <ChannelTestDialog open onOpenChange={() => undefined} />
}

function renderDialog() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })

  render(
    <QueryClientProvider client={queryClient}>
      <ChannelsProvider>
        <Harness channel={channel} />
      </ChannelsProvider>
    </QueryClientProvider>
  )
}

async function findModelRow(model: string) {
  const row = (await screen.findByText(model)).closest('tr')
  expect(row).not.toBeNull()
  return row as HTMLElement
}

describe('channel test dialog model deletion', () => {
  beforeEach(() => {
    vi.mocked(updateChannel).mockReset()
    vi.mocked(updateChannel).mockResolvedValue({ success: true })
    vi.mocked(handleTestChannel).mockReset()
    vi.mocked(handleTestChannel).mockResolvedValue(undefined)
  })

  test('deletes a single model from the row action', async () => {
    const user = userEvent.setup()
    renderDialog()

    const row = await findModelRow('model-a')
    await user.click(within(row).getByRole('button', { name: 'Delete Model' }))

    const confirmation = await screen.findByRole('alertdialog')
    expect(
      within(confirmation).getByText(
        'This removes this model from this channel. This action cannot be undone.'
      )
    ).toBeVisible()

    await user.click(
      within(confirmation).getByRole('button', { name: 'Delete' })
    )

    await waitFor(() => {
      expect(updateChannel).toHaveBeenCalledWith(7, {
        models: 'model-b,model-c',
      })
    })
    await waitFor(() => {
      expect(screen.queryByText('model-a')).not.toBeInTheDocument()
    })
  })

  test('deletes multiple selected models from the bulk action', async () => {
    const user = userEvent.setup()
    renderDialog()

    await findModelRow('model-a')
    await user.click(
      screen.getByRole('checkbox', { name: 'Select model model-a' })
    )
    await user.click(
      screen.getByRole('checkbox', { name: 'Select model model-c' })
    )

    await user.click(
      screen.getByRole('button', { name: 'Delete selected models' })
    )

    const confirmation = await screen.findByRole('alertdialog')
    expect(
      within(confirmation).getByText(
        'This removes 2 models from this channel. This action cannot be undone.'
      )
    ).toBeVisible()

    await user.click(
      within(confirmation).getByRole('button', { name: 'Delete' })
    )

    await waitFor(() => {
      expect(updateChannel).toHaveBeenCalledWith(7, {
        models: 'model-b',
      })
    })
  })

  test('deletes failed models after a failed test', async () => {
    vi.mocked(handleTestChannel).mockImplementation(
      async (_id, _options, onComplete) => {
        onComplete?.(false, undefined, 'upstream failed', 'bad_response')
      }
    )

    const user = userEvent.setup()
    renderDialog()

    const row = await findModelRow('model-a')
    await user.click(
      within(row).getByRole('button', { name: 'Test Connection' })
    )

    await user.click(
      await screen.findByRole('button', {
        name: 'Delete failed models (1)',
      })
    )

    const confirmation = await screen.findByRole('alertdialog')
    expect(
      within(confirmation).getByText(
        'This removes 1 failed models from this channel. This action cannot be undone.'
      )
    ).toBeVisible()

    await user.click(
      within(confirmation).getByRole('button', { name: 'Delete' })
    )

    await waitFor(() => {
      expect(updateChannel).toHaveBeenCalledWith(7, {
        models: 'model-b,model-c',
      })
    })
  })
})
