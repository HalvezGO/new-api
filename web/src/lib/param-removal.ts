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

export const MAX_PARAM_REMOVAL_LIST_SIZE = 100

const PARAM_NAME_PATTERN = /^[A-Za-z0-9_.-]{1,128}$/

export function parseParamNameList(value: string | null | undefined): string[] {
  const trimmed = (value ?? '').trim()
  if (!trimmed) return []
  const seen = new Set<string>()
  const params: string[] = []
  for (const entry of trimmed.split(/[,\n]/)) {
    const name = entry.trim()
    if (!name || seen.has(name)) continue
    seen.add(name)
    params.push(name)
  }
  return params
}

export function serializeParamNameList(
  params: readonly string[] | null | undefined
): string {
  if (!params || params.length === 0) return ''
  return parseParamNameList(params.join(',')).join(', ')
}

export type ParamNameListValidation =
  | { ok: true; params: string[] }
  | { ok: false; invalid: string[]; tooMany: boolean }

export function validateParamNameList(
  value: string | null | undefined
): ParamNameListValidation {
  const params = parseParamNameList(value)
  const invalid = params.filter((param) => !PARAM_NAME_PATTERN.test(param))
  const tooMany = params.length > MAX_PARAM_REMOVAL_LIST_SIZE
  if (invalid.length > 0 || tooMany) {
    return { ok: false, invalid, tooMany }
  }
  return { ok: true, params }
}
