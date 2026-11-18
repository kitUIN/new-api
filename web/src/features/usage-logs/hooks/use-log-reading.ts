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
import { useEffect, useState, type RefObject } from 'react'

function findScrollContainer(element: HTMLElement): HTMLElement {
  let parent = element.parentElement
  while (parent) {
    if (/(auto|scroll)/.test(getComputedStyle(parent).overflowY)) return parent
    parent = parent.parentElement
  }
  return document.scrollingElement as HTMLElement
}

export function useLogReading(
  ref: RefObject<HTMLDivElement | null>,
  enabled: boolean
) {
  const [reading, setReading] = useState(false)
  useEffect(() => {
    const root = ref.current
    if (!root || !enabled) return
    const scroll = findScrollContainer(root)
    let overRow = false
    let disposed = false
    const update = () => {
      if (disposed) return
      setReading(
        scroll.scrollTop > 24 ||
          overRow ||
          Boolean(window.getSelection()?.toString()) ||
          Boolean(
            document.querySelector('[role="dialog"], [role="alertdialog"]')
          )
      )
    }
    const onPointerOver = (event: PointerEvent) => {
      if (event.pointerType !== 'mouse') return
      overRow =
        event.target instanceof Element &&
        Boolean(event.target.closest('.usage-log-row'))
      update()
    }
    const onPointerLeave = () => {
      overRow = false
      update()
    }
    const onFocus = () => queueMicrotask(update)
    scroll.addEventListener('scroll', update, { passive: true })
    root.addEventListener('pointerover', onPointerOver)
    root.addEventListener('pointerleave', onPointerLeave)
    document.addEventListener('selectionchange', update)
    document.addEventListener('focusin', onFocus)
    document.addEventListener('focusout', onFocus)
    update()
    return () => {
      disposed = true
      scroll.removeEventListener('scroll', update)
      root.removeEventListener('pointerover', onPointerOver)
      root.removeEventListener('pointerleave', onPointerLeave)
      document.removeEventListener('selectionchange', update)
      document.removeEventListener('focusin', onFocus)
      document.removeEventListener('focusout', onFocus)
    }
  }, [ref, enabled])

  return {
    reading: enabled && reading,
    scrollToTop: () => {
      if (ref.current)
        findScrollContainer(ref.current).scrollTo({
          top: 0,
          behavior: 'instant',
        })
    },
  }
}
