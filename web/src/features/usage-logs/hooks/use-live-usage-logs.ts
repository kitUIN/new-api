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
import { useEffect } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { SSE } from 'sse.js'
import { useAuthStore } from '@/stores/auth-store'
import { getCommonHeaders } from '@/lib/api'

export function useLiveUsageLogs(enabled: boolean, isAdmin: boolean) {
  const queryClient = useQueryClient()
  const userID = useAuthStore((state) => state.auth.user?.id)

  useEffect(() => {
    if (!enabled || !userID) return
    let source: SSE | undefined
    let refreshTimer: ReturnType<typeof setTimeout> | undefined
    let reconnectTimer: ReturnType<typeof setTimeout> | undefined
    let disposed = false
    let lastRefresh = 0
    let lastStatsRefresh = 0

    const refresh = () => {
      if (document.hidden || refreshTimer || disposed) return
      // Coalesce busy gateways' events without starving a continuous stream.
      refreshTimer = setTimeout(
        () => {
          refreshTimer = undefined
          lastRefresh = Date.now()
          void queryClient.invalidateQueries({ queryKey: ['logs', 'common'] })
          if (lastRefresh - lastStatsRefresh >= 15000) {
            lastStatsRefresh = lastRefresh
            void queryClient.invalidateQueries({
              queryKey: ['usage-logs-stats'],
            })
          }
        },
        Math.max(0, 2000 - (Date.now() - lastRefresh))
      )
    }
    const reconnect = () => {
      const previous = source
      source = undefined
      previous?.close()
      if (disposed || reconnectTimer || document.hidden) return
      reconnectTimer = setTimeout(() => {
        reconnectTimer = undefined
        connect()
      }, 3000)
    }
    const connect = () => {
      if (disposed || document.hidden) return
      source = new SSE(isAdmin ? '/api/log/events' : '/api/log/self/events', {
        method: 'GET',
        headers: getCommonHeaders(),
        withCredentials: true,
        start: false,
      })
      source.addEventListener('changed', refresh)
      source.addEventListener('reconnect', reconnect)
      source.addEventListener('error', reconnect)
      source.addEventListener(
        'readystatechange',
        (event: { readyState: number }) => {
          if (event.readyState === 2) reconnect()
        }
      )
      source.stream()
    }
    const onVisibility = () => {
      if (document.hidden) {
        source?.close()
      } else {
        if (reconnectTimer) clearTimeout(reconnectTimer)
        reconnectTimer = undefined
        connect()
        refresh()
      }
    }
    connect()
    // Repair missed pub/sub messages and support deployments without shared Redis.
    const fallback = setInterval(refresh, 15000)
    document.addEventListener('visibilitychange', onVisibility)
    return () => {
      disposed = true
      source?.close()
      if (refreshTimer) clearTimeout(refreshTimer)
      if (reconnectTimer) clearTimeout(reconnectTimer)
      clearInterval(fallback)
      document.removeEventListener('visibilitychange', onVisibility)
    }
  }, [enabled, isAdmin, userID, queryClient])
}
