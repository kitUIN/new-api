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
import assert from 'node:assert/strict'
import test from 'node:test'
import {
  DRAWING_ASPECT_RATIOS,
  DRAWING_RESOLUTIONS,
  resolveDrawingSize,
} from '../constants'
import {
  isDrawingSizeAligned,
  resolveDrawingSizeSelection,
} from './size-selection'

test('each reference or uploaded image can supply the output size', () => {
  const images = ['reference', 'upload-a', 'upload-b']
  const sizes = ['640x480', '2048x2736', '4096x4096']
  for (const [index, originalImage] of images.entries()) {
    const result = resolveDrawingSizeSelection({
      images,
      sizes,
      originalImage,
      aspectRatio: '1:1',
      resolution: '1K',
    })
    assert.equal(result.selectedAspectRatio, `image-${index}`)
    assert.equal(result.size, sizes[index])
  }
})

test('removing an earlier image preserves the selected image identity', () => {
  const result = resolveDrawingSizeSelection({
    images: ['reference', 'upload-b'],
    sizes: ['640x480', '4096x4096'],
    originalImage: 'upload-b',
    aspectRatio: '1:1',
    resolution: '2K',
  })
  assert.equal(result.selectedAspectRatio, 'image-1')
  assert.equal(result.size, '4096x4096')
})

test('removing the selected image returns to the preset instead of a different image', () => {
  const result = resolveDrawingSizeSelection({
    images: ['reference', 'upload-b'],
    sizes: ['640x480', '4096x4096'],
    originalImage: 'upload-a',
    aspectRatio: '1:1',
    resolution: '2K',
  })
  assert.equal(result.usesOriginalSize, false)
  assert.equal(result.size, '2048x2048')
})

test('unloaded original dimensions cannot silently submit a preset size', () => {
  const result = resolveDrawingSizeSelection({
    images: ['reference'],
    sizes: [undefined],
    originalImage: 'reference',
    aspectRatio: '1:1',
    resolution: '1K',
  })
  assert.equal(result.size, '')
})

test('4K presets retain 4096 on the short edge', () => {
  for (const ratio of ['1:1', '9:16', '16:9', '3:4', '4:3'] as const) {
    const dimensions = resolveDrawingSize(ratio, '4K').split('x').map(Number)
    assert.equal(Math.min(...dimensions), 4096)
  }
})

test('both dimensions must be positive integer multiples of 16', () => {
  for (const size of ['16x16', '640x480', '2048x2736', '4096x5456']) {
    assert.equal(isDrawingSizeAligned(size), true, size)
  }
  for (const size of [
    undefined,
    '',
    'auto',
    '0x16',
    '-16x16',
    '16x0',
    '16x17',
    '17x16',
    '1024x1365',
    '2048x2731',
    '4096x5461',
    '16.0x16',
    '16x16x16',
  ]) {
    assert.equal(isDrawingSizeAligned(size), false, size)
  }
})

test('an invalid original size is blocked without rounding the uploaded image dimensions', () => {
  for (const size of ['1025x1024', '1024x1025', '2048x2731']) {
    const result = resolveDrawingSizeSelection({
      images: ['reference'],
      sizes: [size],
      originalImage: 'reference',
      aspectRatio: '1:1',
      resolution: '1K',
    })
    assert.equal(result.originalSize, size)
    assert.equal(result.size, '')
  }
})

test('every preset can be selected with both dimensions aligned to 16', () => {
  for (const ratio of DRAWING_ASPECT_RATIOS) {
    for (const resolution of DRAWING_RESOLUTIONS) {
      const size = resolveDrawingSize(ratio.value, resolution.value)
      assert.equal(
        isDrawingSizeAligned(size),
        true,
        `${ratio.value} ${resolution.value}: ${size}`
      )
    }
  }
})
