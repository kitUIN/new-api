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
import { DRAWING_SESSION_TITLE_PREFIX } from '../constants'
import type { DrawingSession } from '../types'

export function getNextDrawingSessionTitle(sessions: DrawingSession[]): string {
  const usedIndexes = new Set<number>()

  for (const session of sessions) {
    const title = String(session.title || '').trim()
    if (!title.startsWith(DRAWING_SESSION_TITLE_PREFIX)) continue

    const suffix = title.slice(DRAWING_SESSION_TITLE_PREFIX.length)
    if (!/^\d+$/.test(suffix)) continue

    const index = Number(suffix)
    if (Number.isSafeInteger(index) && index > 0) {
      usedIndexes.add(index)
    }
  }

  for (let index = 1; ; index += 1) {
    if (!usedIndexes.has(index)) {
      return `${DRAWING_SESSION_TITLE_PREFIX}${index}`
    }
  }
}
