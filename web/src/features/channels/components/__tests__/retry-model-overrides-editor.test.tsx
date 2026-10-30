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
import { fireEvent, render, screen } from '@testing-library/react'
import { useState } from 'react'
import { describe, expect, test } from 'vitest'

import type { RetryModelOverrideRow } from '../../lib'
import { RetryModelOverridesEditor } from '../retry-model-overrides-editor'

function Harness(props: {
  initialRows: RetryModelOverrideRow[]
  disabled?: boolean
}) {
  const [rows, setRows] = useState(props.initialRows)
  return (
    <RetryModelOverridesEditor
      rows={rows}
      modelOptions={['model-a', 'model-b', 'model-c']}
      onChange={setRows}
      disabled={props.disabled}
    />
  )
}

const modelARow: RetryModelOverrideRow = {
  id: 'row-a',
  model: 'model-a',
  retryTimes: '3',
  retryInterval: '1500',
}

describe('RetryModelOverridesEditor', () => {
  test('adds an empty override row', () => {
    render(<Harness initialRows={[]} />)

    expect(
      screen.getByText(
        'No model retry overrides configured. Click "Add Override" to get started.'
      )
    ).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Add Override' }))

    expect(screen.getAllByRole('combobox')).toHaveLength(1)
    expect(
      (screen.getByLabelText('Retry Times') as HTMLInputElement).value
    ).toBe('')
    expect(
      (screen.getByLabelText('Retry Interval (ms)') as HTMLInputElement).value
    ).toBe('')
  })

  test('copies retry values into a new row without a model', () => {
    render(<Harness initialRows={[modelARow]} />)

    fireEvent.click(screen.getByRole('button', { name: 'Copy override' }))

    const comboboxes = screen.getAllByRole('combobox') as HTMLInputElement[]
    expect(comboboxes).toHaveLength(2)
    expect(comboboxes[1].value).toBe('')

    const retryTimes = screen.getAllByLabelText(
      'Retry Times'
    ) as HTMLInputElement[]
    expect(retryTimes.map((input) => input.value)).toEqual(['3', '3'])

    const retryIntervals = screen.getAllByLabelText(
      'Retry Interval (ms)'
    ) as HTMLInputElement[]
    expect(retryIntervals.map((input) => input.value)).toEqual(['1500', '1500'])
  })

  test('deletes an override row', () => {
    render(<Harness initialRows={[modelARow]} />)

    fireEvent.click(screen.getByRole('button', { name: 'Delete override' }))

    expect(screen.queryAllByRole('combobox')).toHaveLength(0)
  })

  test('excludes models already used by other rows from the dropdown', () => {
    render(
      <Harness
        initialRows={[
          modelARow,
          { id: 'row-empty', model: '', retryTimes: '', retryInterval: '' },
        ]}
      />
    )

    const comboboxes = screen.getAllByRole('combobox')
    const openButtons = screen.getAllByRole('button', { name: 'Open' })
    fireEvent.click(openButtons[1])
    expect(comboboxes).toHaveLength(2)

    expect(
      screen.queryByRole('option', { name: 'model-a' })
    ).not.toBeInTheDocument()
    expect(screen.getByRole('option', { name: 'model-b' })).toBeInTheDocument()
  })

  test('disables the model picker and actions when requested', () => {
    render(<Harness initialRows={[modelARow]} disabled />)

    expect(screen.getByRole('combobox')).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Add Override' })).toBeDisabled()
    expect(
      screen.getByRole('button', { name: 'Delete override' })
    ).toBeDisabled()
  })
})
