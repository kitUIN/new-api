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
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import type { RefObject } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useLocation, useNavigate } from '@tanstack/react-router'
import { PlusIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useAuthStore } from '@/stores/auth-store'
import { getSelf } from '@/lib/api'
import { useStatus } from '@/hooks/use-status'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { getPricing } from '@/features/pricing/api'
import { getDrawingMessageImages } from './api'
import { DrawingCanvas, DrawingInputBar, SessionSelector } from './components'
import { DEFAULT_DRAWING_MODEL } from './constants'
import {
  useDrawingMessages,
  useDrawingSessions,
  useDrawingSubmit,
} from './hooks'
import { buildDrawingBalanceInfo } from './lib/balance'
import {
  extractDrawingResultImages,
  getDrawingImageSource,
  mergeDrawingImages,
} from './lib/images'
import { getNextDrawingSessionTitle } from './lib/sessions'
import type {
  DrawingGenerateRequest,
  DrawingMessage,
  DrawingSession,
} from './types'

export function Drawing() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const location = useLocation()
  const { status } = useStatus()
  const authUser = useAuthStore((state) => state.auth.user)
  const setUser = useAuthStore((state) => state.auth.setUser)
  const [titleDraft, setTitleDraft] = useState('')
  const [titleDraftEdited, setTitleDraftEdited] = useState(false)
  const [titleEditing, setTitleEditing] = useState(false)
  const [referenceImage, setReferenceImage] = useState('')
  const [retrying, setRetrying] = useState(false)
  const [deleteTarget, setDeleteTarget] = useState<DrawingSession | null>(null)
  const urlMessageTargetRef = useRef('')
  const newDraftRef = useRef(false)

  const urlParams = useMemo(
    () => new URLSearchParams(location.searchStr),
    [location.searchStr]
  )
  const urlSessionId = urlParams.get('session')
  const urlMessageId = urlParams.get('message')

  const sessionsState = useDrawingSessions()
  const messagesState = useDrawingMessages(sessionsState.activeSessionId)
  const submitState = useDrawingSubmit({
    activeSessionId: sessionsState.activeSessionId,
    addOptimisticMessage: messagesState.addOptimisticMessage,
    updateMessageByTaskId: messagesState.updateMessageByTaskId,
  })

  const currentMessage = messagesState.currentMessage
  const nextDefaultTitle = useMemo(
    () => getNextDrawingSessionTitle(sessionsState.sessions),
    [sessionsState.sessions]
  )

  const pricingQuery = useQuery({
    queryKey: ['drawing', 'pricing'],
    queryFn: getPricing,
  })

  useEffect(() => {
    void getSelf().then((res) => {
      if (res?.success && res.data) setUser(res.data)
    })
  }, [setUser])

  useEffect(() => {
    if (newDraftRef.current) {
      if (!urlSessionId) newDraftRef.current = false
      return
    }

    if (urlSessionId) {
      sessionsState.setActiveSessionId((prev) =>
        prev === urlSessionId ? prev : urlSessionId
      )
    }
  }, [sessionsState.setActiveSessionId, urlSessionId])

  useEffect(() => {
    if (!sessionsState.activeSessionId) {
      urlMessageTargetRef.current = ''
      void messagesState.loadLatestMessage()
      return
    }

    const urlMessageTarget =
      urlSessionId === sessionsState.activeSessionId && urlMessageId
        ? `${sessionsState.activeSessionId}:${urlMessageId}`
        : ''

    if (urlMessageTarget) {
      if (urlMessageTargetRef.current !== urlMessageTarget) {
        urlMessageTargetRef.current = urlMessageTarget
        if (urlMessageId) void messagesState.loadCurrentMessage(urlMessageId)
      }
      return
    }

    urlMessageTargetRef.current = ''
    if (
      !currentMessage ||
      currentMessage.session_id !== sessionsState.activeSessionId
    ) {
      void messagesState.loadLatestMessage()
    }
  }, [
    currentMessage,
    messagesState.loadCurrentMessage,
    messagesState.loadLatestMessage,
    sessionsState.activeSessionId,
    urlMessageId,
    urlSessionId,
  ])

  useEffect(() => {
    return () => submitState.stopAllPolling()
  }, [submitState.stopAllPolling])

  useEffect(() => {
    if (
      currentMessage?.task_id &&
      (currentMessage.status === 'pending' ||
        currentMessage.status === 'processing')
    ) {
      submitState.startPolling(currentMessage.task_id)
    }
  }, [currentMessage?.status, currentMessage?.task_id, submitState])

  useEffect(() => {
    if (currentMessage?.status === 'success') {
      void sessionsState.loadSessions()
    }
  }, [currentMessage?.id, currentMessage?.status, sessionsState.loadSessions])

  useEffect(() => {
    if (
      !sessionsState.activeSessionId &&
      urlSessionId &&
      !newDraftRef.current
    ) {
      return
    }

    void syncUrl({
      activeSessionId: sessionsState.activeSessionId,
      currentSearch: location.searchStr,
      currentMessage,
      navigate,
      urlMessageId,
      urlMessageTargetRef,
    })
  }, [
    currentMessage,
    location.searchStr,
    navigate,
    sessionsState.activeSessionId,
    urlMessageId,
    urlSessionId,
  ])

  useEffect(() => {
    let ignore = false

    async function loadReferenceImage() {
      const images = await resolveReferenceImages(
        currentMessage,
        sessionsState.activeSessionId
      )
      if (!ignore) setReferenceImage(images[0] || '')
    }

    if (!currentMessage || currentMessage.status !== 'success') {
      setReferenceImage('')
      return () => {
        ignore = true
      }
    }

    void loadReferenceImage()
    return () => {
      ignore = true
    }
  }, [
    currentMessage?.id,
    currentMessage?.result_data,
    currentMessage?.status,
    sessionsState.activeSessionId,
  ])

  useEffect(() => {
    const activeSession = sessionsState.sessions.find(
      (session) => session.session_id === sessionsState.activeSessionId
    )
    if (sessionsState.activeSessionId) {
      setTitleDraft(activeSession?.title || nextDefaultTitle)
      setTitleDraftEdited(false)
      setTitleEditing(false)
      return
    }
    if (!titleDraftEdited) setTitleDraft(nextDefaultTitle)
  }, [
    nextDefaultTitle,
    sessionsState.activeSessionId,
    sessionsState.sessions,
    titleDraftEdited,
  ])

  const pricingModel = useMemo(() => {
    const models = pricingQuery.data?.data || []
    return (
      models.find((model) => model.model_name === DEFAULT_DRAWING_MODEL) || null
    )
  }, [pricingQuery.data?.data])

  const balanceInfo = useMemo(
    () =>
      buildDrawingBalanceInfo({
        userQuota: authUser?.quota,
        quotaPerUnit: Number(status?.quota_per_unit || 1),
        model: pricingModel,
        groupRatio: pricingQuery.data?.group_ratio || {},
        pricingLoading: pricingQuery.isLoading,
      }),
    [
      authUser?.quota,
      pricingModel,
      pricingQuery.data?.group_ratio,
      pricingQuery.isLoading,
      status?.quota_per_unit,
    ]
  )

  const displayTitle = titleDraft || nextDefaultTitle
  const isLoading = messagesState.messages.some(
    (message) => message.status === 'pending' || message.status === 'processing'
  )

  const handleNewSession = useCallback(() => {
    newDraftRef.current = true
    urlMessageTargetRef.current = ''
    sessionsState.setActiveSessionId(null)
    messagesState.setCurrentMessage(null)
    setTitleDraft(nextDefaultTitle)
    setTitleDraftEdited(false)
    setTitleEditing(false)
    void navigate({ to: '/drawing', search: {}, replace: true })
  }, [
    messagesState.setCurrentMessage,
    navigate,
    nextDefaultTitle,
    sessionsState.setActiveSessionId,
  ])

  const handleSelectSession = useCallback(
    (sessionId: string) => {
      sessionsState.setActiveSessionId(sessionId)
      setTitleDraftEdited(false)
    },
    [sessionsState.setActiveSessionId]
  )

  const handleSubmit = useCallback(
    async (payload: DrawingGenerateRequest) => {
      let sessionId = sessionsState.activeSessionId
      if (!sessionId) {
        const title = titleDraftEdited ? titleDraft.trim() : ''
        const session = await sessionsState.createSession(title)
        if (!session) return
        newDraftRef.current = false
        sessionId = session.session_id
        setTitleDraft(session.title || title || nextDefaultTitle)
        setTitleDraftEdited(false)
        setTitleEditing(false)
      }

      const referenceImages = await resolveReferenceImages(
        currentMessage,
        sessionId
      )
      await submitState.submit(
        {
          ...payload,
          images: mergeDrawingImages(referenceImages, payload.images),
        },
        sessionId
      )
    },
    [
      currentMessage,
      nextDefaultTitle,
      sessionsState,
      submitState,
      titleDraft,
      titleDraftEdited,
    ]
  )

  const handleRetry = useCallback(
    (message: DrawingMessage) => {
      if (!message.prompt.trim() || retrying) return
      setRetrying(true)
      submitState
        .submit(
          {
            prompt: message.prompt.trim(),
            model: message.model || DEFAULT_DRAWING_MODEL,
            size: message.size || '1024x1024',
            quality: message.quality || 'auto',
            images: [],
          },
          message.session_id || sessionsState.activeSessionId
        )
        .finally(() => setRetrying(false))
    },
    [retrying, sessionsState.activeSessionId, submitState]
  )

  const handleSaveTitle = useCallback(async () => {
    const nextTitle = titleDraft.trim() || displayTitle
    if (!sessionsState.activeSessionId) {
      setTitleDraft(nextTitle)
      setTitleDraftEdited(Boolean(titleDraft.trim()))
      setTitleEditing(false)
      return
    }
    await sessionsState.renameSession(sessionsState.activeSessionId, nextTitle)
    setTitleEditing(false)
  }, [displayTitle, sessionsState, titleDraft])

  const handleDeleteConfirmed = useCallback(async () => {
    if (!deleteTarget?.session_id) return
    const isActive = deleteTarget.session_id === sessionsState.activeSessionId
    if (isActive) {
      newDraftRef.current = true
      urlMessageTargetRef.current = ''
      sessionsState.setActiveSessionId(null)
      messagesState.setCurrentMessage(null)
      void navigate({ to: '/drawing', search: {}, replace: true })
    }
    await sessionsState.removeSession(deleteTarget.session_id)
    setDeleteTarget(null)
  }, [deleteTarget, messagesState, navigate, sessionsState])

  return (
    <div className='bg-background flex size-full min-h-0 flex-col overflow-hidden'>
      <div className='flex shrink-0 items-center justify-between gap-3 border-b px-3 py-3 sm:px-4'>
        <div className='flex min-w-0 items-center gap-2'>
          <SessionSelector
            activeSessionId={sessionsState.activeSessionId}
            loading={sessionsState.loading}
            onCreate={handleNewSession}
            onDelete={setDeleteTarget}
            onSelect={handleSelectSession}
            sessions={sessionsState.sessions}
          />

          {titleEditing ? (
            <Input
              autoFocus
              className='h-9 w-[min(56vw,28rem)]'
              maxLength={200}
              onBlur={handleSaveTitle}
              onChange={(event) => {
                setTitleDraft(event.target.value)
                setTitleDraftEdited(true)
              }}
              onKeyDown={(event) => {
                if (event.key === 'Enter') void handleSaveTitle()
                if (event.key === 'Escape') setTitleEditing(false)
              }}
              value={titleDraft}
            />
          ) : (
            <Button
              className='max-w-[min(56vw,28rem)] justify-start truncate px-3'
              onClick={() => setTitleEditing(true)}
              title={displayTitle}
              type='button'
              variant='outline'
            >
              <span className='truncate'>{displayTitle}</span>
            </Button>
          )}
        </div>

        <Button
          aria-label={t('New session')}
          onClick={handleNewSession}
          size='icon'
          type='button'
          variant='outline'
        >
          <PlusIcon className='size-4' />
        </Button>
      </div>

      <div className='min-h-0 flex-1 overflow-hidden'>
        <DrawingCanvas
          activeSessionId={sessionsState.activeSessionId}
          loading={messagesState.loading}
          messages={messagesState.messages}
          onLoadNext={messagesState.loadNextMessage}
          onLoadPrevious={messagesState.loadPreviousMessage}
          onRetry={handleRetry}
          pageInfo={messagesState.pageInfo}
          retryDisabled={isLoading || retrying}
        />
      </div>

      <DrawingInputBar
        balanceInfo={balanceInfo}
        disabled={false}
        hasImage={messagesState.messages.some(
          (message) => message.status === 'success'
        )}
        loading={isLoading}
        onSubmit={handleSubmit}
        referenceImage={referenceImage}
      />

      <AlertDialog
        open={Boolean(deleteTarget)}
        onOpenChange={(open) => {
          if (!open) setDeleteTarget(null)
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('Delete session')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('Delete this drawing session and its generated images?')}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('Cancel')}</AlertDialogCancel>
            <AlertDialogAction
              className='bg-destructive text-destructive-foreground hover:bg-destructive/90'
              onClick={handleDeleteConfirmed}
            >
              {t('Delete')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}

async function syncUrl(options: {
  activeSessionId: string | null
  currentSearch: string
  currentMessage: DrawingMessage | null
  navigate: ReturnType<typeof useNavigate>
  urlMessageId: string | null
  urlMessageTargetRef: RefObject<string>
}) {
  if (!options.activeSessionId) {
    if (!options.currentSearch) return
    await options.navigate({ to: '/drawing', search: {}, replace: true })
    return
  }

  const pendingTarget =
    options.activeSessionId && options.urlMessageId
      ? `${options.activeSessionId}:${options.urlMessageId}`
      : ''
  if (
    pendingTarget &&
    options.urlMessageTargetRef.current === pendingTarget &&
    (!options.currentMessage ||
      String(options.currentMessage.id || '') !== options.urlMessageId)
  ) {
    return
  }

  const search: Record<string, string> = {
    session: options.activeSessionId,
  }
  if (
    options.currentMessage?.id &&
    options.currentMessage.session_id === options.activeSessionId &&
    !options.currentMessage.optimistic
  ) {
    search.message = String(options.currentMessage.id)
  }

  const nextSearch = new URLSearchParams(search).toString()
  if (options.currentSearch.replace(/^\?/, '') === nextSearch) return

  await options.navigate({ to: '/drawing', search, replace: true })
}

async function resolveReferenceImages(
  message: DrawingMessage | null,
  sessionId: string | null
): Promise<string[]> {
  if (!message || message.status !== 'success' || !sessionId) return []

  const fromMessage = extractDrawingResultImages(message.result_data)
    .map(getDrawingImageSource)
    .filter(Boolean)
  if (fromMessage.length > 0) return fromMessage

  const data = await getDrawingMessageImages(sessionId, message.id).catch(
    () => null
  )
  return extractDrawingResultImages(data?.result_data)
    .map(getDrawingImageSource)
    .filter(Boolean)
}
