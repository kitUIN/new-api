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
import { api } from '@/lib/api'
import type {
  ApiEnvelope,
  DrawingGenerateRequest,
  DrawingMessageImages,
  DrawingMessagePage,
  DrawingSession,
  DrawingTaskStatus,
} from './types'

const DRAWING_API = {
  sessions: '/pg/drawing/sessions',
  sessionDetail: (sessionId: string) => `/pg/drawing/sessions/${sessionId}`,
  sessionMessage: (sessionId: string) =>
    `/pg/drawing/sessions/${sessionId}/message`,
  messageImages: (sessionId: string, messageId: number) =>
    `/pg/drawing/sessions/${sessionId}/messages/${messageId}/images`,
  generate: (sessionId: string) => `/pg/drawing/sessions/${sessionId}/generate`,
  taskStatus: (taskId: string) => `/pg/drawing/tasks/${taskId}`,
} as const

export async function listDrawingSessions(): Promise<DrawingSession[]> {
  const res = await api.get<ApiEnvelope<DrawingSession[]>>(DRAWING_API.sessions)
  return res.data.success ? res.data.data || [] : []
}

export async function createDrawingSession(
  title?: string
): Promise<DrawingSession | null> {
  const payload = title?.trim() ? { title: title.trim() } : {}
  const res = await api.post<ApiEnvelope<DrawingSession>>(
    DRAWING_API.sessions,
    payload
  )
  return res.data.success ? res.data.data : null
}

export async function updateDrawingSessionTitle(
  sessionId: string,
  title: string
): Promise<boolean> {
  const res = await api.patch<ApiEnvelope<{ title: string }>>(
    DRAWING_API.sessionDetail(sessionId),
    { title }
  )
  return res.data.success
}

export async function deleteDrawingSession(
  sessionId: string
): Promise<boolean> {
  const res = await api.delete<ApiEnvelope<null>>(
    DRAWING_API.sessionDetail(sessionId)
  )
  return res.data.success
}

export async function getDrawingMessage(
  sessionId: string,
  direction: 'latest' | 'current' | 'prev' | 'next',
  currentId?: number | string
): Promise<DrawingMessagePage | null> {
  const res = await api.get<ApiEnvelope<DrawingMessagePage>>(
    DRAWING_API.sessionMessage(sessionId),
    {
      params: {
        direction,
        ...(currentId ? { current_id: currentId } : {}),
      },
    }
  )
  return res.data.success ? res.data.data : null
}

export async function getDrawingMessageImages(
  sessionId: string,
  messageId: number
): Promise<DrawingMessageImages | null> {
  const res = await api.get<ApiEnvelope<DrawingMessageImages>>(
    DRAWING_API.messageImages(sessionId, messageId)
  )
  return res.data.success ? res.data.data : null
}

export async function generateDrawing(
  sessionId: string,
  payload: DrawingGenerateRequest
): Promise<{ task_id: string; message_id: number } | null> {
  const res = await api.post<
    ApiEnvelope<{ task_id: string; message_id: number }>
  >(DRAWING_API.generate(sessionId), payload)
  return res.data.success ? res.data.data : null
}

export async function getDrawingTaskStatus(
  taskId: string
): Promise<DrawingTaskStatus | null> {
  const res = await api.get<ApiEnvelope<DrawingTaskStatus>>(
    DRAWING_API.taskStatus(taskId),
    { disableDuplicate: true } as Record<string, unknown>
  )
  return res.data.success ? res.data.data : null
}
