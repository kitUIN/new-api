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
import { MAX_UPLOAD_IMAGES } from '../constants'
import type { DrawingImageResult, DrawingMessage } from '../types'

export function parseDrawingImages(value: unknown): string[] {
  if (!value) return []
  if (Array.isArray(value)) return value.filter(isString)
  if (typeof value !== 'string') return []

  try {
    const parsed = JSON.parse(value) as unknown
    return Array.isArray(parsed) ? parsed.filter(isString) : []
  } catch {
    return []
  }
}

export function extractDrawingResultImages(
  value: unknown
): DrawingImageResult[] {
  if (!value) return []
  if (Array.isArray(value)) return value.filter(isImageResult)
  if (typeof value !== 'string') return []

  try {
    const parsed = JSON.parse(value) as unknown
    return Array.isArray(parsed) ? parsed.filter(isImageResult) : []
  } catch {
    return []
  }
}

export function getDrawingImageSource(image: DrawingImageResult): string {
  const url = image.url || image.b64_json || ''
  if (!url) return ''
  if (url.startsWith('/')) return url
  if (url.startsWith('data:')) return url
  if (/^https?:\/\//i.test(url)) return url
  return `data:image/png;base64,${url}`
}

export function mergeDrawingImages(
  referenceImages: string[],
  uploadedImages: string[]
): string[] {
  const merged: string[] = []
  for (const image of [...referenceImages, ...uploadedImages]) {
    if (!image || merged.includes(image)) continue
    merged.push(image)
    if (merged.length >= MAX_UPLOAD_IMAGES) break
  }
  return merged
}

export function getMessageResultImageUrls(message: DrawingMessage | null) {
  return extractDrawingResultImages(message?.result_data)
    .map(getDrawingImageSource)
    .filter(Boolean)
    .slice(0, MAX_UPLOAD_IMAGES)
}

function isString(value: unknown): value is string {
  return typeof value === 'string' && value.length > 0
}

function isImageResult(value: unknown): value is DrawingImageResult {
  if (!value || typeof value !== 'object') return false
  const item = value as Record<string, unknown>
  return typeof item.url === 'string' || typeof item.b64_json === 'string'
}
