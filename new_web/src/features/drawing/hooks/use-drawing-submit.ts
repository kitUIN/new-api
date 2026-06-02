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
import { useCallback, useRef } from 'react'
import { generateDrawing, getDrawingTaskStatus } from '../api'
import { POLL_INTERVAL, POLL_TIMEOUT } from '../constants'
import type { DrawingGenerateRequest, DrawingMessage } from '../types'

type UseDrawingSubmitOptions = {
  activeSessionId: string | null
  addOptimisticMessage: (message: DrawingMessage) => void
  updateMessageByTaskId: (
    taskId: string | null,
    updates: Partial<DrawingMessage>
  ) => void
}

export function useDrawingSubmit(options: UseDrawingSubmitOptions) {
  const { activeSessionId, addOptimisticMessage, updateMessageByTaskId } =
    options
  const pollTimersRef = useRef<Record<string, ReturnType<typeof setTimeout>>>(
    {}
  )

  const startPolling = useCallback(
    (taskId: string) => {
      if (!taskId || pollTimersRef.current[taskId]) return

      const startTime = Date.now()

      const poll = async () => {
        if (Date.now() - startTime > POLL_TIMEOUT) {
          updateMessageByTaskId(taskId, {
            status: 'failure',
            fail_reason: 'Polling timed out',
          })
          delete pollTimersRef.current[taskId]
          return
        }

        try {
          const data = await getDrawingTaskStatus(taskId)
          if (data?.status === 'SUCCESS') {
            updateMessageByTaskId(taskId, {
              status: 'success',
              result_data: data.result_data,
            })
            delete pollTimersRef.current[taskId]
            return
          }

          if (data?.status === 'FAILURE') {
            updateMessageByTaskId(taskId, {
              status: 'failure',
              fail_reason: data.fail_reason || 'Generation failed',
            })
            delete pollTimersRef.current[taskId]
            return
          }
        } catch {
          /* keep polling; transient network errors are expected */
        }

        pollTimersRef.current[taskId] = setTimeout(poll, POLL_INTERVAL)
      }

      pollTimersRef.current[taskId] = setTimeout(poll, POLL_INTERVAL)
    },
    [updateMessageByTaskId]
  )

  const submit = useCallback(
    async (payload: DrawingGenerateRequest, sessionId?: string | null) => {
      const targetSessionId = sessionId || activeSessionId
      if (!targetSessionId || !payload.prompt.trim()) return null

      const optimisticMessage: DrawingMessage = {
        id: Date.now(),
        session_id: targetSessionId,
        task_id: null,
        prompt: payload.prompt,
        model: payload.model,
        size: payload.size,
        quality: payload.quality,
        image_urls: payload.images,
        status: 'pending',
        created_at: Math.floor(Date.now() / 1000),
        optimistic: true,
      }
      addOptimisticMessage(optimisticMessage)

      try {
        const data = await generateDrawing(targetSessionId, payload)
        if (!data?.task_id) {
          updateMessageByTaskId(null, {
            status: 'failure',
            fail_reason: 'Generation request failed',
          })
          return optimisticMessage.id
        }

        updateMessageByTaskId(null, {
          id: data.message_id || optimisticMessage.id,
          task_id: data.task_id,
          status: 'processing',
          optimistic: false,
        })
        startPolling(data.task_id)
      } catch (error) {
        updateMessageByTaskId(null, {
          status: 'failure',
          fail_reason:
            error instanceof Error
              ? error.message
              : 'Generation request failed',
        })
      }

      return optimisticMessage.id
    },
    [activeSessionId, addOptimisticMessage, startPolling, updateMessageByTaskId]
  )

  const stopAllPolling = useCallback(() => {
    Object.values(pollTimersRef.current).forEach(clearTimeout)
    pollTimersRef.current = {}
  }, [])

  return { submit, startPolling, stopAllPolling }
}
