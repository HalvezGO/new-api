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
import { Copy, Plus, Trash2 } from 'lucide-react'
import { useRef } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Combobox } from '@/components/ui/combobox'
import { Input } from '@/components/ui/input'
import { Skeleton } from '@/components/ui/skeleton'

import {
  MAX_CHANNEL_RETRY_INTERVAL,
  MAX_CHANNEL_RETRY_TIMES,
  type RetryModelOverrideRow,
} from '../lib'

type RetryModelOverridesEditorProps = {
  rows: RetryModelOverrideRow[]
  modelOptions: string[]
  onChange: (rows: RetryModelOverrideRow[]) => void
  disabled?: boolean
  loading?: boolean
}

export function RetryModelOverridesEditor(
  props: RetryModelOverridesEditorProps
) {
  const { t } = useTranslation()
  const nextRowIdRef = useRef(0)

  const createRowId = () => {
    nextRowIdRef.current += 1
    return `retry-override-${nextRowIdRef.current}`
  }

  const updateRow = (
    id: string,
    patch: Partial<Omit<RetryModelOverrideRow, 'id'>>
  ) => {
    props.onChange(
      props.rows.map((row) => (row.id === id ? { ...row, ...patch } : row))
    )
  }

  const handleAddRow = () => {
    props.onChange([
      ...props.rows,
      {
        id: createRowId(),
        model: '',
        retryTimes: '',
        retryInterval: '',
      },
    ])
  }

  const handleCopyRow = (row: RetryModelOverrideRow) => {
    props.onChange([
      ...props.rows,
      {
        id: createRowId(),
        model: '',
        retryTimes: row.retryTimes,
        retryInterval: row.retryInterval,
      },
    ])
  }

  const handleDeleteRow = (id: string) => {
    props.onChange(props.rows.filter((row) => row.id !== id))
  }

  const modelOptions = [...new Set(props.modelOptions)]

  const getModelOptions = (row: RetryModelOverrideRow) => {
    const usedByOtherRows = new Set(
      props.rows
        .filter((otherRow) => otherRow.id !== row.id)
        .map((otherRow) => otherRow.model)
        .filter(Boolean)
    )
    return modelOptions
      .filter((model) => !usedByOtherRows.has(model))
      .map((model) => ({ value: model, label: model }))
  }

  const usedModels = new Set(props.rows.map((row) => row.model).filter(Boolean))
  const allModelsUsed =
    modelOptions.length > 0 &&
    modelOptions.every((model) => usedModels.has(model))

  if (props.loading) {
    return <Skeleton className='h-24 w-full' />
  }

  return (
    <div className='space-y-2'>
      {props.rows.length > 0 ? (
        <div className='space-y-2'>
          <div className='grid grid-cols-[minmax(0,1fr)_6.5rem_7.5rem_auto] gap-2 text-sm font-medium'>
            <div>{t('Model')}</div>
            <div>{t('Retry Times')}</div>
            <div>{t('Retry Interval (ms)')}</div>
            <div className='w-[4.5rem]' aria-hidden='true' />
          </div>
          {props.rows.map((row) => (
            <div
              key={row.id}
              className='grid grid-cols-[minmax(0,1fr)_6.5rem_7.5rem_auto] items-center gap-2'
            >
              <Combobox
                options={getModelOptions(row)}
                value={row.model}
                onValueChange={(value) =>
                  updateRow(row.id, { model: value ?? '' })
                }
                placeholder={t('Select Model')}
                searchPlaceholder={t('Search models...')}
                emptyText={t('No model found.')}
                disabled={props.disabled}
                openOnFocus={false}
              />
              <Input
                type='number'
                min={0}
                max={MAX_CHANNEL_RETRY_TIMES}
                value={row.retryTimes}
                placeholder={t('Inherit')}
                aria-label={t('Retry Times')}
                disabled={props.disabled}
                onChange={(event) =>
                  updateRow(row.id, { retryTimes: event.target.value })
                }
              />
              <Input
                type='number'
                min={0}
                max={MAX_CHANNEL_RETRY_INTERVAL}
                value={row.retryInterval}
                placeholder={t('Inherit')}
                aria-label={t('Retry Interval (ms)')}
                disabled={props.disabled}
                onChange={(event) =>
                  updateRow(row.id, { retryInterval: event.target.value })
                }
              />
              <div className='flex items-center gap-1'>
                <Button
                  type='button'
                  variant='ghost'
                  size='icon'
                  className='h-8 w-8'
                  onClick={() => handleCopyRow(row)}
                  disabled={props.disabled || allModelsUsed}
                  aria-label={t('Copy override')}
                >
                  <Copy className='h-4 w-4' aria-hidden='true' />
                </Button>
                <Button
                  type='button'
                  variant='ghost'
                  size='icon'
                  className='h-8 w-8'
                  onClick={() => handleDeleteRow(row.id)}
                  disabled={props.disabled}
                  aria-label={t('Delete override')}
                >
                  <Trash2 className='h-4 w-4' aria-hidden='true' />
                </Button>
              </div>
            </div>
          ))}
        </div>
      ) : (
        <div className='text-muted-foreground flex h-20 items-center justify-center rounded-md border border-dashed px-3 text-center text-sm'>
          {modelOptions.length === 0
            ? t('Add models to this channel first.')
            : t(
                'No model retry overrides configured. Click "Add Override" to get started.'
              )}
        </div>
      )}
      <Button
        type='button'
        variant='outline'
        size='sm'
        className='w-full'
        onClick={handleAddRow}
        disabled={props.disabled || modelOptions.length === 0 || allModelsUsed}
      >
        <Plus className='mr-2 h-4 w-4' aria-hidden='true' />
        {t('Add Override')}
      </Button>
    </div>
  )
}
