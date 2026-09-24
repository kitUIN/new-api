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
import { useCallback, useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import {
  createDrawingSession,
  deleteDrawingSession,
  listDrawingSessions,
  updateDrawingSessionTitle,
} from '../api'
import type { DrawingSession } from '../types'

export function useDrawingSessions() {
  const { t } = useTranslation()
  const [sessions, setSessions] = useState<DrawingSession[]>([])
  const [activeSessionId, setActiveSessionId] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)

  const loadSessions = useCallback(async () => {
    setLoading(true)
    try {
      setSessions(await listDrawingSessions())
    } catch {
      toast.error(t('Failed to load drawing sessions'))
    } finally {
      setLoading(false)
    }
  }, [t])

  const createSession = useCallback(
    async (title?: string) => {
      try {
        const session = await createDrawingSession(title)
        if (!session) return null
        setSessions((prev) => [session, ...prev])
        setActiveSessionId(session.session_id)
        return session
      } catch {
        toast.error(t('Failed to create drawing session'))
        return null
      }
    },
    [t]
  )

  const removeSession = useCallback(
    async (sessionId: string) => {
      try {
        const success = await deleteDrawingSession(sessionId)
        if (!success) return false
        setSessions((prev) =>
          prev.filter((session) => session.session_id !== sessionId)
        )
        setActiveSessionId((prev) => (prev === sessionId ? null : prev))
        return true
      } catch {
        toast.error(t('Failed to delete drawing session'))
        return false
      }
    },
    [t]
  )

  const renameSession = useCallback(
    async (sessionId: string, title: string) => {
      const nextTitle = title.trim()
      if (!sessionId || !nextTitle) return false

      const previousSessions = sessions
      setSessions((prev) =>
        prev.map((session) =>
          session.session_id === sessionId
            ? { ...session, title: nextTitle }
            : session
        )
      )

      try {
        const success = await updateDrawingSessionTitle(sessionId, nextTitle)
        if (success) return true
      } catch {
        toast.error(t('Failed to rename drawing session'))
      }

      setSessions(previousSessions)
      return false
    },
    [sessions, t]
  )

  useEffect(() => {
    void loadSessions()
  }, [loadSessions])

  return {
    sessions,
    activeSessionId,
    setActiveSessionId,
    loading,
    createSession,
    removeSession,
    renameSession,
    loadSessions,
  }
}
