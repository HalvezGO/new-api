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
import { useMediaQuery } from '@/hooks'
import { formatTimestampToDate } from '@/lib/format'

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
        description={t(
          'Only failed attempts are listed. The log row shows the key used by the final attempt.'
        )}
        contentClassName='sm:max-w-4xl'
        contentHeight='min(65vh, 620px)'
        bodyClassName='flex h-full min-h-0 flex-col gap-3'
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
  const isMobile = useMediaQuery('(max-width: 640px)')

  const columns = useMemo<ColumnDef<RetryFailureEntry>[]>(() => {
    const adminColumns: ColumnDef<RetryFailureEntry>[] = props.isAdmin
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
          {
            id: 'key',
            header: t('API Key'),
            cell: ({ row }) => {
              const label = formatAttemptedKey(row.original)
              return label ? (
                <TruncatedCell tooltipContent={label}>
                  <span className='font-mono text-xs'>{label}</span>
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
      {
        id: 'attempted_at',
        header: t('Time'),
        cell: ({ row }) => {
          const attemptedAt = row.original.attempted_at
          return attemptedAt ? (
            <span className='font-mono text-xs tabular-nums'>
              {formatTimestampToDate(attemptedAt)}
            </span>
          ) : (
            <span className='text-muted-foreground'>-</span>
          )
        },
      },
      ...adminColumns,
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
    <div className='flex min-h-0 flex-1 flex-col gap-3'>
      <DataTableView
        table={table}
        containerClassName='min-h-0 flex-1 rounded-md'
        containerProps={{
          role: 'region',
          'aria-label': t('Retry history'),
        }}
        tableContainerClassName='h-full min-h-0'
        tableHeaderClassName='[background-color:var(--table-header)]'
        splitHeaderScrollClassName='h-full'
        bodyContainerClassName='[scrollbar-gutter:stable]'
        splitHeader
        tableClassName='w-full table-auto'
        colgroup={
          <colgroup>
            <col className='w-14' />
            <col className='w-40' />
            {props.isAdmin ? <col className='w-48' /> : null}
            {props.isAdmin ? <col className='w-44' /> : null}
            <col className='w-24' />
            <col className='w-40' />
            <col className='w-24' />
            <col />
          </colgroup>
        }
        emptyContent={t('No data')}
        emptyCellClassName='text-muted-foreground h-16 text-center text-sm'
      />
      <DataTablePagination table={table} compact={isMobile} />
    </div>
  )
}

function formatAttemptedKey(failure: RetryFailureEntry): string {
  const parts: string[] = []
  if (failure.key_index !== undefined) {
    parts.push(`#${failure.key_index + 1}`)
  }
  const masked = failure.key_masked?.trim()
  if (masked) parts.push(masked)
  return parts.join(' · ')
}

function formatChannel(failure: RetryFailureEntry): string {
  const id = failure.channel_id
  const name = failure.channel_name?.trim()
  if (id && name) return `#${id} (${name})`
  if (id) return `#${id}`
  return name ?? ''
}
