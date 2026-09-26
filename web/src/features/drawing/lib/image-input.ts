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
export function readDrawingImageSize(source: string): Promise<string> {
  return new Promise((resolve, reject) => {
    const image = new Image()
    const timeout = window.setTimeout(() => finish(), 15000)

    function finish(size?: string) {
      window.clearTimeout(timeout)
      image.onload = null
      image.onerror = null
      if (size) resolve(size)
      else reject(new Error('Failed to read image'))
    }

    image.onload = () => {
      if (!image.naturalWidth || !image.naturalHeight) {
        finish()
        return
      }
      finish(`${image.naturalWidth}x${image.naturalHeight}`)
    }
    image.onerror = () => finish()
    image.src = source
  })
}

export function readDrawingFile(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onerror = () => reject(new Error('Failed to read image'))
    reader.onabort = () => reject(new Error('Failed to read image'))
    reader.onload = async () => {
      const source = reader.result
      if (typeof source !== 'string') {
        reject(new Error('Failed to read image'))
        return
      }
      try {
        await readDrawingImageSize(source)
        resolve(source)
      } catch (error) {
        reject(error)
      }
    }
    reader.readAsDataURL(file)
  })
}
