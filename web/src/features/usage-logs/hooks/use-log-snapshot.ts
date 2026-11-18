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
import { useState } from 'react'
import {
  advanceLogSnapshot,
  createLogSnapshot,
  type SnapshotInput,
} from '../lib/live-snapshot'

export function useLogSnapshot(input: SnapshotInput) {
  const [snapshot, setSnapshot] = useState(() => createLogSnapshot(input))
  const next = advanceLogSnapshot(snapshot, input)
  // Derive the visible snapshot before rendering children, so a background
  // response never flashes the new ordering for one frame.
  if (next !== snapshot) setSnapshot(next)
  return {
    data: next.display ?? input.data,
    pendingCount: next.pendingCount,
    hasUpdates: next.hasUpdates,
    acceptLatest: () => setSnapshot(createLogSnapshot(input)),
  }
}
