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
import { useTranslation } from 'react-i18next'

import type { RetryFailureEntry } from '../types'

interface RetryHistoryListProps {
  failures: RetryFailureEntry[]
  /** True when the request had more failed attempts than the backend kept. */
  truncated?: boolean
  /**
   * Attempt channel identity is admin-only: the channel column is admin-only and
   * the API strips channel ids/names from user-facing payloads, so the list must
   * not put them back on screen for non-admins.
   */
  isAdmin: boolean
}

export function RetryHistoryList(props: RetryHistoryListProps) {
  const { t } = useTranslation()
  if (props.failures.length === 0) return null

  return (
    <div className='space-y-1'>
      <div className='text-muted-foreground text-xs'>
        {t('Retry history')} ·{' '}
        {t('Retried {{count}} attempts', { count: props.failures.length })}
        {props.truncated ? '…' : ''}
      </div>
      <div className='space-y-0.5'>
        {props.failures.map((failure) => (
          <div
            key={`${failure.retry_index}-${failure.channel_id ?? 'x'}`}
            className='text-muted-foreground/80 font-mono text-[11px] leading-snug'
          >
            #{failure.retry_index}
            {props.isAdmin && (
              <>
                {' · ch '}
                {failure.channel_id}
                {failure.channel_name ? ` (${failure.channel_name})` : ''}
              </>
            )}
            {' · '}
            {failure.status_code}
            {failure.waited_ms ? ` · waited ${failure.waited_ms}ms` : ''} —{' '}
            {failure.reason}
          </div>
        ))}
      </div>
    </div>
  )
}
