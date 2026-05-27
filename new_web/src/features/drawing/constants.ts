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
export const DEFAULT_DRAWING_MODEL = 'gpt-image-2'

export const DRAWING_ASPECT_RATIOS = [
  { value: '1:1', label: 'Square' },
  { value: '9:16', label: '9:16 Portrait' },
  { value: '16:9', label: '16:9 Landscape' },
  { value: '3:4', label: '3:4 Portrait' },
  { value: '4:3', label: '4:3 Landscape' },
] as const

export const DRAWING_RESOLUTIONS = [
  { value: '1K', label: '1K' },
  { value: '2K', label: '2K' },
] as const

export const DRAWING_SIZE_MAP = {
  '1K': {
    '1:1': '1024x1024',
    '9:16': '1024x1792',
    '16:9': '1792x1024',
    '3:4': '1024x1365',
    '4:3': '1365x1024',
  },
  '2K': {
    '1:1': '2048x2048',
    '9:16': '2048x3584',
    '16:9': '3584x2048',
    '3:4': '2048x2731',
    '4:3': '2731x2048',
  },
} as const

export const MAX_UPLOAD_IMAGES = 4
export const POLL_INTERVAL = 30000
export const POLL_TIMEOUT = 300000
export const DRAWING_SESSION_TITLE_PREFIX = 'New Session '

export function resolveDrawingSize(
  aspectRatio: keyof (typeof DRAWING_SIZE_MAP)['1K'],
  resolution: keyof typeof DRAWING_SIZE_MAP
): string {
  return DRAWING_SIZE_MAP[resolution]?.[aspectRatio] || '1024x1024'
}
