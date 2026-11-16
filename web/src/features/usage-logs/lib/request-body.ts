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
import type { RequestBodyInfo } from '../types'

export function formatBodySize(
  size: number | undefined,
  exact = false
): string | null {
  if (size === undefined || !Number.isFinite(size) || size < 0) return null
  const bytes = `${size.toLocaleString()} B`
  if (size < 1024) return bytes
  const units = ['KiB', 'MiB', 'GiB', 'TiB']
  const index = Math.min(Math.floor(Math.log2(size) / 10) - 1, units.length - 1)
  const compact = `${(size / 1024 ** (index + 1)).toFixed(2)} ${units[index]}`
  return exact ? `${compact} (${bytes})` : compact
}

export function getRawRequestBodySize(
  info?: RequestBodyInfo
): number | undefined {
  if (info?.raw_size !== undefined) return info.raw_size
  // Older uncompressed logs already contain the original payload length.
  if (info?.encoding === 'identity') return info.size
  return undefined
}
