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
import { useEffect, useRef, useState } from 'react'
import {
  AlertCircleIcon,
  ChevronLeftIcon,
  ChevronRightIcon,
  ImageIcon,
  RotateCcwIcon,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { Spinner } from '@/components/ui/spinner'
import { getDrawingMessageImages } from '../api'
import { extractDrawingResultImages } from '../lib/images'
import type { DrawingImageResult, DrawingMessage } from '../types'
import { DrawingImageCard } from './drawing-image-card'

type DrawingCanvasProps = {
  activeSessionId: string | null
  loading: boolean
  messages: DrawingMessage[]
  pageInfo: {
    current_index: number
    total: number
    has_prev: boolean
    has_next: boolean
  }
  retryDisabled: boolean
  onLoadPrevious: () => void
  onLoadNext: () => void
  onRetry: (message: DrawingMessage) => void
}

export function DrawingCanvas(props: DrawingCanvasProps) {
  const { t } = useTranslation()
  const [imageCache, setImageCache] = useState<
    Record<number, DrawingImageResult[]>
  >({})
  const [imageLoading, setImageLoading] = useState(false)
  const fetchedRef = useRef(new Set<number>())
  const message = props.messages[0] || null
  const isGenerating =
    message?.status === 'pending' || message?.status === 'processing'

  useEffect(() => {
    fetchedRef.current = new Set()
    setImageCache({})
  }, [props.activeSessionId])

  useEffect(() => {
    if (!message || message.status !== 'success') return
    if (imageCache[message.id] || fetchedRef.current.has(message.id)) return

    fetchedRef.current.add(message.id)
    setImageLoading(true)
    getDrawingMessageImages(message.session_id, message.id)
      .then((data) => {
        setImageCache((prev) => ({
          ...prev,
          [message.id]: extractDrawingResultImages(data?.result_data),
        }))
      })
      .catch(() => undefined)
      .finally(() => setImageLoading(false))
  }, [imageCache, message])

  if (!props.activeSessionId) {
    return <EmptyDrawingState text={t('Enter a prompt to generate an image')} />
  }

  if (props.loading && !message) {
    return (
      <div className='flex h-full items-center justify-center'>
        <Spinner className='text-primary size-6' />
      </div>
    )
  }

  if (!message) {
    return <EmptyDrawingState text={t('Enter a prompt to generate an image')} />
  }

  const resultImages =
    imageCache[message.id] || extractDrawingResultImages(message.result_data)

  return (
    <div className='flex h-full min-h-0 flex-col'>
      {props.pageInfo.total > 1 && (
        <div className='flex shrink-0 items-center justify-center gap-2 px-3 py-2'>
          <Button
            aria-label={t('Previous image')}
            disabled={props.loading || !props.pageInfo.has_prev}
            onClick={props.onLoadPrevious}
            size='icon-sm'
            type='button'
            variant='ghost'
          >
            <ChevronLeftIcon className='size-4' />
          </Button>
          <span className='text-muted-foreground min-w-12 text-center text-xs'>
            {props.pageInfo.current_index} / {props.pageInfo.total}
          </span>
          <Button
            aria-label={t('Next image')}
            disabled={props.loading || !props.pageInfo.has_next}
            onClick={props.onLoadNext}
            size='icon-sm'
            type='button'
            variant='ghost'
          >
            <ChevronRightIcon className='size-4' />
          </Button>
        </div>
      )}

      <div className='flex min-h-0 flex-1 flex-col items-center gap-4 overflow-auto overscroll-contain px-4 py-4 sm:px-6'>
        {isGenerating && (
          <div className='m-auto flex max-w-lg flex-col items-center gap-3 text-center'>
            <Spinner className='text-primary size-7' />
            <span className='text-primary text-sm'>
              {t('Image generation may take several minutes. Please wait.')}
            </span>
            <p className='text-muted-foreground line-clamp-3 text-xs'>
              {message.prompt}
            </p>
          </div>
        )}

        {message.status === 'failure' && (
          <div className='m-auto flex max-w-lg flex-col items-center gap-3 text-center'>
            <div className='border-destructive/30 bg-destructive/10 text-destructive flex items-center gap-2 rounded-lg border px-4 py-3 text-sm'>
              <AlertCircleIcon className='size-4' />
              <span>{message.fail_reason || t('Generation failed')}</span>
            </div>
            <Button
              disabled={props.retryDisabled}
              onClick={() => props.onRetry(message)}
              type='button'
              variant='outline'
            >
              <RotateCcwIcon className='size-4' />
              {t('Retry')}
            </Button>
          </div>
        )}

        {message.status === 'success' && (
          <>
            {imageLoading && resultImages.length === 0 ? (
              <div className='m-auto'>
                <Spinner className='text-primary size-6' />
              </div>
            ) : (
              <div className='m-auto grid w-full max-w-4xl grid-cols-1 gap-3 md:grid-cols-[repeat(auto-fit,minmax(min(100%,18rem),1fr))]'>
                {resultImages.map((image, index) => (
                  <DrawingImageCard
                    image={image}
                    key={`${message.id}-${index}`}
                  />
                ))}
              </div>
            )}
          </>
        )}

        {!isGenerating && message.prompt && (
          <p className='text-muted-foreground line-clamp-3 max-w-2xl text-center text-xs'>
            {message.prompt}
          </p>
        )}
      </div>
    </div>
  )
}

function EmptyDrawingState(props: { text: string }) {
  return (
    <div className='flex h-full flex-col items-center justify-center gap-4 px-6 text-center'>
      <div className='bg-muted flex size-16 items-center justify-center rounded-full'>
        <ImageIcon className='text-muted-foreground size-7' />
      </div>
      <p className='text-muted-foreground text-sm'>{props.text}</p>
    </div>
  )
}
