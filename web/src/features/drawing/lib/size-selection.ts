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
import { resolveDrawingSize } from '../constants'

type SizeSelection = {
  images: string[]
  sizes: (string | undefined)[]
  originalImage: string
  aspectRatio: Parameters<typeof resolveDrawingSize>[0]
  resolution: Parameters<typeof resolveDrawingSize>[1]
}

export function isDrawingSizeAligned(size: string | undefined): boolean {
  if (!size || !/^\d+x\d+$/.test(size)) return false
  return size.split('x').every((dimension) => {
    const value = Number(dimension)
    return Number.isSafeInteger(value) && value > 0 && value % 16 === 0
  })
}

export function resolveDrawingSizeSelection(input: SizeSelection) {
  const index = input.images.indexOf(input.originalImage)
  const usesOriginalSize = index >= 0
  const originalSize = input.sizes[index]
  const size = usesOriginalSize
    ? originalSize
    : resolveDrawingSize(input.aspectRatio, input.resolution)
  return {
    usesOriginalSize,
    originalSize,
    selectedAspectRatio: usesOriginalSize
      ? `image-${index}`
      : input.aspectRatio,
    size: size && isDrawingSizeAligned(size) ? size : '',
  }
}
