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
import { useRef, useState, type ClipboardEvent, type DragEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { readDrawingFile } from '../lib/image-input'

export function useDrawingAttachments(options: {
  disabled: boolean
  maxImages: number
}) {
  const { t } = useTranslation()
  const [images, setImages] = useState<string[]>([])
  const [reading, setReading] = useState(false)
  const [dragging, setDragging] = useState(false)
  const readingRef = useRef(false)
  const dragDepth = useRef(0)

  async function addFiles(files: File[]) {
    if (options.disabled || readingRef.current || files.length === 0) return
    const imageFiles = files.filter((file) => file.type.startsWith('image/'))
    if (imageFiles.length !== files.length) {
      toast.warning(t('Only image files are supported'))
    }
    if (imageFiles.length === 0) return
    if (images.length + imageFiles.length > options.maxImages) {
      toast.warning(
        t('You can upload up to {{count}} images', { count: options.maxImages })
      )
      return
    }

    readingRef.current = true
    setReading(true)
    try {
      const sources = await Promise.all(imageFiles.map(readDrawingFile))
      setImages((previous) => [...previous, ...sources])
    } catch {
      toast.error(t('Failed to read image'))
    } finally {
      readingRef.current = false
      setReading(false)
    }
  }

  function onPaste(event: ClipboardEvent<HTMLDivElement>) {
    const files = Array.from(event.clipboardData.items)
      .filter((item) => item.kind === 'file' && item.type.startsWith('image/'))
      .map((item) => item.getAsFile())
      .filter((file): file is File => file !== null)
    if (files.length === 0) return
    event.preventDefault()
    void addFiles(files)
  }

  function onDragEnter(event: DragEvent<HTMLDivElement>) {
    if (!event.dataTransfer.types.includes('Files')) return
    event.preventDefault()
    dragDepth.current += 1
    if (!options.disabled && !readingRef.current) setDragging(true)
  }

  function onDragOver(event: DragEvent<HTMLDivElement>) {
    if (!event.dataTransfer.types.includes('Files')) return
    event.preventDefault()
    event.dataTransfer.dropEffect =
      options.disabled || reading ? 'none' : 'copy'
  }

  function onDragLeave(event: DragEvent<HTMLDivElement>) {
    event.preventDefault()
    dragDepth.current = Math.max(0, dragDepth.current - 1)
    if (dragDepth.current === 0) setDragging(false)
  }

  function onDrop(event: DragEvent<HTMLDivElement>) {
    if (!event.dataTransfer.types.includes('Files')) return
    event.preventDefault()
    dragDepth.current = 0
    setDragging(false)
    void addFiles(Array.from(event.dataTransfer.files))
  }

  return {
    images,
    setImages,
    reading,
    dragging,
    addFiles,
    inputHandlers: { onPaste, onDragEnter, onDragOver, onDragLeave, onDrop },
  }
}
