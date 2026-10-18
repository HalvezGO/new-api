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

/**
 * Bounds for the usage-logs auto-refresh interval, in seconds. The lower bound
 * keeps a typo from hammering the logs API; the upper bound keeps the control
 * from drifting into "effectively off" territory, where the switch is clearer.
 */
export const AUTO_REFRESH_MIN_SECONDS = 1
export const AUTO_REFRESH_MAX_SECONDS = 300
export const AUTO_REFRESH_DEFAULT_SECONDS = 5

/**
 * Normalize a raw number-input value into a usable refresh interval: empty,
 * non-numeric, or fractional input falls back to the default or is truncated,
 * and everything is clamped into [min, max].
 */
export function clampAutoRefreshSeconds(value: unknown): number {
  // An empty input means "no value yet" (the user cleared the box), which is
  // different from an explicit 0: the former restores the default, the latter
  // is clamped up to the minimum.
  if (value == null || (typeof value === 'string' && value.trim() === '')) {
    return AUTO_REFRESH_DEFAULT_SECONDS
  }
  const parsed = typeof value === 'number' ? value : Number(value)
  if (!Number.isFinite(parsed)) {
    return AUTO_REFRESH_DEFAULT_SECONDS
  }
  const whole = Math.trunc(parsed)
  return Math.min(
    AUTO_REFRESH_MAX_SECONDS,
    Math.max(AUTO_REFRESH_MIN_SECONDS, whole)
  )
}
