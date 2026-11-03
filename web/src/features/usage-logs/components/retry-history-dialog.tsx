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
import type { ColumnDef } from '@tanstack/react-table'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import {
  DataTablePagination,
  DataTableView,
  TruncatedCell,
  useDataTable,
} from '@/components/data-table'
import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'

import type { RetryFailureEntry } from '../types'

interface RetryHistoryDialogProps {
  failures: RetryFailureEntry[]
  /** True when the request had more failed attempts than the backend kept. */
  truncated?: boolean
  /**
   * Attempt channel identity is admin-only: the channel column is admin-only and
   * the API strips channel ids/names from user-facing payloads, so the table must
   * not put them back on screen for non-admins.
   */
  isAdmin: boolean
}

export function RetryHistoryDialog(props: RetryHistoryDialogProps) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)

  if (props.failures.length === 0) return null

  return (
    <>
      <Button
        variant='link'
        size='xs'
        className='h-auto max-w-full p-0 text-start whitespace-normal'
        onClick={() => setOpen(true)}
      >
        {t('Retried {{count}} attempts', { count: props.failures.length })}
        {props.truncated ? '…' : ''}
      </Button>

      <Dialog
        open={open}
        onOpenChange={setOpen}
        title={t('Retry history')}
        contentClassName='sm:max-w-4xl'
        contentHeight='auto'
        bodyClassName='space-y-3'
        footer={
          <Button variant='outline' onClick={() => setOpen(false)}>
            {t('Close')}
          </Button>
        }
      >
        {props.truncated ? (
          <p className='text-muted-foreground text-xs'>
            {t('Only the first {{count}} attempts are kept.', {
              count: props.failures.length,
            })}
          </p>
        ) : null}

        <RetryHistoryTable failures={props.failures} isAdmin={props.isAdmin} />
      </Dialog>
    </>
  )
}

function RetryHistoryTable(props: {
  failures: RetryFailureEntry[]
  isAdmin: boolean
}) {
  const { t } = useTranslation()

  const columns = useMemo<ColumnDef<RetryFailureEntry>[]>(() => {
    const channelColumns: ColumnDef<RetryFailureEntry>[] = props.isAdmin
      ? [
          {
            id: 'channel',
            header: t('Channel'),
            cell: ({ row }) => {
              const channel = formatChannel(row.original)
              return channel ? (
                <TruncatedCell tooltipContent={channel}>
                  {channel}
                </TruncatedCell>
              ) : (
                <span className='text-muted-foreground'>-</span>
              )
            },
          },
        ]
      : []

    return [
      {
        id: 'retry_index',
        header: '#',
        cell: ({ row }) => (
          <span className='font-mono tabular-nums'>
            #{row.original.retry_index}
          </span>
        ),
      },
      ...channelColumns,
      {
        id: 'status_code',
        header: t('Status Code'),
        cell: ({ row }) => (
          <span className='font-mono tabular-nums'>
            {row.original.status_code}
          </span>
        ),
      },
      {
        id: 'error_code',
        header: t('Error code'),
        cell: ({ row }) =>
          row.original.error_code ? (
            <TruncatedCell tooltipContent={row.original.error_code}>
              <span className='font-mono text-xs'>
                {row.original.error_code}
              </span>
            </TruncatedCell>
          ) : (
            <span className='text-muted-foreground'>-</span>
          ),
      },
      {
        id: 'waited_ms',
        header: t('Waiting'),
        cell: ({ row }) =>
          row.original.waited_ms > 0 ? `${row.original.waited_ms}ms` : '-',
      },
      {
        id: 'reason',
        header: t('Reason'),
        cell: ({ row }) => (
          <TruncatedCell tooltipContent={row.original.reason}>
            <span className='text-xs'>{row.original.reason}</span>
          </TruncatedCell>
        ),
      },
    ]
  }, [props.isAdmin, t])

  const { table } = useDataTable({
    data: props.failures,
    columns,
    initialPagination: { pageIndex: 0, pageSize: 10 },
    enableSorting: false,
    withFilteredRowModel: false,
    withSortedRowModel: false,
    withFacetedRowModel: false,
  })

  return (
    <div className='space-y-3'>
      <DataTableView
        table={table}
        containerClassName='rounded-md'
        containerProps={{
          role: 'region',
          'aria-label': t('Retry history'),
        }}
        tableContainerClassName='max-h-[55vh] overflow-auto'
        tableClassName='w-full table-auto'
        colgroup={
          <colgroup>
            <col className='w-14' />
            {props.isAdmin ? <col className='w-48' /> : null}
            <col className='w-24' />
            <col className='w-40' />
            <col className='w-24' />
            <col />
          </colgroup>
        }
        emptyContent={t('No data')}
        emptyCellClassName='text-muted-foreground h-16 text-center text-sm'
      />
      <DataTablePagination table={table} compact />
    </div>
  )
}

function formatChannel(failure: RetryFailureEntry): string {
  const id = failure.channel_id
  const name = failure.channel_name?.trim()
  if (id && name) return `#${id} (${name})`
  if (id) return `#${id}`
  return name ?? ''
}
