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
import { create } from 'zustand'

type NavigationRequest = {
  url: string
  target: string
  proceed: () => void
  cancel: () => void
}

type NavigationConfirmationState = {
  pending: NavigationRequest | null
  request: (url: string, target: string, action: () => void) => Promise<boolean>
  confirm: () => void
  cancel: () => void
}

export const useNavigationConfirmationStore =
  create<NavigationConfirmationState>((set, get) => ({
    pending: null,
    request: (url, target, action) => {
      // Keep the destination stable while the user is reviewing it.
      if (get().pending) return Promise.resolve(false)

      return new Promise<boolean>((resolve, reject) => {
        set({
          pending: {
            url,
            target,
            proceed: () => {
              try {
                // Run in the confirmation click, before resolving the promise,
                // so opening a tab retains the browser's user activation.
                action()
                resolve(true)
              } catch (error) {
                reject(error)
              }
            },
            cancel: () => resolve(false),
          },
        })
      })
    },
    confirm: () => {
      const pending = get().pending
      set({ pending: null })
      pending?.proceed()
    },
    cancel: () => {
      const pending = get().pending
      set({ pending: null })
      pending?.cancel()
    },
  }))
