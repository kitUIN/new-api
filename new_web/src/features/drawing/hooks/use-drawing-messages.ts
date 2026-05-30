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
import { useCallback, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { getDrawingMessage } from '../api'
import type { DrawingMessage, DrawingMessagePage } from '../types'

const EMPTY_PAGE_INFO = {
  current_index: 0,
  total: 0,
  has_prev: false,
  has_next: false,
}

export function useDrawingMessages(activeSessionId: string | null) {
  const { t } = useTranslation()
  const [currentMessage, setCurrentMessage] = useState<DrawingMessage | null>(
    null
  )
  const [pageInfo, setPageInfo] = useState(EMPTY_PAGE_INFO)
  const [loading, setLoading] = useState(false)
  const requestIdRef = useRef(0)

  const applyResponse = useCallback((data: DrawingMessagePage | null) => {
    setCurrentMessage(data?.message || null)
    setPageInfo({
      current_index: data?.current_index || 0,
      total: data?.total || 0,
      has_prev: Boolean(data?.has_prev),
      has_next: Boolean(data?.has_next),
    })
  }, [])

  const resetMessages = useCallback(() => {
    requestIdRef.current += 1
    setCurrentMessage(null)
    setPageInfo(EMPTY_PAGE_INFO)
    setLoading(false)
  }, [])

  const loadMessage = useCallback(
    async (
      direction: 'latest' | 'current' | 'prev' | 'next' = 'latest',
      currentId?: number | string
    ) => {
      const requestId = requestIdRef.current + 1
      requestIdRef.current = requestId

      if (!activeSessionId) {
        setCurrentMessage(null)
        setPageInfo(EMPTY_PAGE_INFO)
        setLoading(false)
        return
      }

      setLoading(true)
      try {
        const data = await getDrawingMessage(
          activeSessionId,
          direction,
          currentId
        )
        if (requestId === requestIdRef.current) {
          applyResponse(data)
        }
      } catch {
        if (requestId === requestIdRef.current) {
          toast.error(t('Failed to load drawing message'))
        }
      } finally {
        if (requestId === requestIdRef.current) setLoading(false)
      }
    },
    [activeSessionId, applyResponse, t]
  )

  const loadLatestMessage = useCallback(
    () => loadMessage('latest'),
    [loadMessage]
  )

  const loadCurrentMessage = useCallback(
    (messageId: string) => {
      if (!messageId) return
      return loadMessage('current', messageId)
    },
    [loadMessage]
  )

  const loadPreviousMessage = useCallback(() => {
    if (!currentMessage?.id || !pageInfo.has_prev) return
    return loadMessage('prev', currentMessage.id)
  }, [currentMessage?.id, loadMessage, pageInfo.has_prev])

  const loadNextMessage = useCallback(() => {
    if (!currentMessage?.id || !pageInfo.has_next) return
    return loadMessage('next', currentMessage.id)
  }, [currentMessage?.id, loadMessage, pageInfo.has_next])

  const addOptimisticMessage = useCallback((message: DrawingMessage) => {
    setCurrentMessage(message)
    setPageInfo((prev) => {
      const total = prev.total + 1
      return {
        current_index: total,
        total,
        has_prev: total > 1,
        has_next: false,
      }
    })
  }, [])

  const updateMessageByTaskId = useCallback(
    (taskId: string | null, updates: Partial<DrawingMessage>) => {
      setCurrentMessage((prev) => {
        if (!prev) return prev
        if (taskId !== null && prev.task_id !== taskId) return prev
        if (taskId === null && prev.task_id) return prev
        return { ...prev, ...updates }
      })
    },
    []
  )

  return {
    messages: currentMessage ? [currentMessage] : [],
    currentMessage,
    pageInfo,
    loading,
    loadLatestMessage,
    loadCurrentMessage,
    loadPreviousMessage,
    loadNextMessage,
    addOptimisticMessage,
    updateMessageByTaskId,
    resetMessages,
  }
}
