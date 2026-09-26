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
import { useMemo, useRef, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { ImagePlusIcon, SendIcon, XIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'
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
import { Dialog, DialogContent, DialogTitle } from '@/components/ui/dialog'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'
import {
  DEFAULT_DRAWING_MODEL,
  DRAWING_ASPECT_RATIOS,
  DRAWING_RESOLUTIONS,
  MAX_UPLOAD_IMAGES,
  resolveDrawingSize,
} from '../constants'
import { useDrawingAttachments } from '../hooks/use-drawing-attachments'
import { readDrawingImageSize } from '../lib/image-input'
import { mergeDrawingImages } from '../lib/images'
import type { DrawingBalanceInfo, DrawingGenerateRequest } from '../types'
import { BalancePopover } from './balance-popover'

type AspectRatio = (typeof DRAWING_ASPECT_RATIOS)[number]['value']
type Resolution = (typeof DRAWING_RESOLUTIONS)[number]['value']

const DRAWING_ASPECT_RATIO_STORAGE_KEY = 'drawing:aspect-ratio'
const DRAWING_RESOLUTION_STORAGE_KEY = 'drawing:resolution'
const DRAWING_ASPECT_RATIO_VALUES = DRAWING_ASPECT_RATIOS.map(
  (item) => item.value
)
const DRAWING_RESOLUTION_VALUES = DRAWING_RESOLUTIONS.map((item) => item.value)

type DrawingInputBarProps = {
  balanceInfo: DrawingBalanceInfo
  disabled: boolean
  hasImage: boolean
  loading: boolean
  referenceImages: string[]
  onSubmit: (payload: DrawingGenerateRequest) => Promise<void>
}

export function DrawingInputBar(props: DrawingInputBarProps) {
  const { t } = useTranslation()
  const [prompt, setPrompt] = useState('')
  const [aspectRatio, setAspectRatio] = useState<AspectRatio | 'original'>(() =>
    getStoredDrawingPreference(
      DRAWING_ASPECT_RATIO_STORAGE_KEY,
      DRAWING_ASPECT_RATIO_VALUES,
      '1:1'
    )
  )
  const [resolution, setResolution] = useState<Resolution>(() =>
    getStoredDrawingPreference(
      DRAWING_RESOLUTION_STORAGE_KEY,
      DRAWING_RESOLUTION_VALUES,
      '1K'
    )
  )
  const [previewImage, setPreviewImage] = useState('')
  const [confirmOpen, setConfirmOpen] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const fileInputRef = useRef<HTMLInputElement | null>(null)
  const hasPrompt = prompt.trim().length > 0
  const isSubmitting = props.loading || submitting
  const referenceImages = props.referenceImages
  const maxUploadImages = Math.max(
    0,
    MAX_UPLOAD_IMAGES - referenceImages.length
  )
  const attachments = useDrawingAttachments({
    disabled: props.disabled || isSubmitting,
    maxImages: maxUploadImages,
  })
  const { images, setImages } = attachments
  const inputImages = mergeDrawingImages(referenceImages, images)
  const singleImage = inputImages.length === 1 ? inputImages[0] : ''
  const originalSizeQuery = useQuery({
    queryKey: ['drawing-image-size', singleImage],
    queryFn: () => readDrawingImageSize(singleImage),
    enabled: Boolean(singleImage),
    retry: false,
    staleTime: Infinity,
    gcTime: 0,
  })
  const originalSize = singleImage ? originalSizeQuery.data : undefined
  const selectedAspectRatio =
    aspectRatio === 'original' && !singleImage ? '1:1' : aspectRatio
  const usesOriginalSize = selectedAspectRatio === 'original'
  const size = usesOriginalSize
    ? originalSize || ''
    : resolveDrawingSize(selectedAspectRatio, resolution)
  const canSubmit =
    hasPrompt &&
    !props.disabled &&
    !isSubmitting &&
    !attachments.reading &&
    images.length <= maxUploadImages &&
    Boolean(size)

  const payload = useMemo<DrawingGenerateRequest>(
    () => ({
      prompt: prompt.trim(),
      model: DEFAULT_DRAWING_MODEL,
      size,
      quality: 'auto',
      images,
    }),
    [images, prompt, size]
  )

  const handleConfirmSubmit = async () => {
    if (!canSubmit) return
    setSubmitting(true)
    try {
      await props.onSubmit(payload)
      setPrompt('')
      setImages([])
    } finally {
      setSubmitting(false)
      setConfirmOpen(false)
    }
  }

  return (
    <div className='mx-auto w-full max-w-4xl px-3 pb-3 sm:px-4 sm:pb-4'>
      {props.hasImage && hasPrompt && (
        <div className='mb-2 rounded-lg border border-amber-500/30 bg-amber-500/10 px-3 py-2 text-xs text-amber-700 dark:text-amber-300'>
          {t('This submission will edit the current image.')}
        </div>
      )}

      <div
        className={cn(
          'bg-background ring-border relative rounded-xl ring-1',
          attachments.dragging && 'ring-primary ring-2'
        )}
        {...attachments.inputHandlers}
      >
        {attachments.dragging && (
          <div className='bg-background/90 pointer-events-none absolute inset-0 z-10 flex items-center justify-center rounded-xl text-sm'>
            {t('Drop images here')}
          </div>
        )}
        {(referenceImages.length > 0 || images.length > 0) && (
          <div className='flex flex-wrap gap-2 px-4 pt-3'>
            {referenceImages.map((image, index) => (
              <button
                aria-label={t('Preview image')}
                className='border-primary focus-visible:ring-ring size-14 cursor-zoom-in overflow-hidden rounded-lg border-2 focus-visible:ring-2'
                key={`reference-${index}`}
                onClick={() => setPreviewImage(image)}
                type='button'
              >
                <img
                  alt={t('Reference image')}
                  className='size-full object-cover'
                  draggable={false}
                  src={image}
                />
              </button>
            ))}
            {images.map((image, index) => (
              <div
                className='relative size-14'
                key={`${image.slice(0, 24)}-${index}`}
              >
                <button
                  aria-label={t('Preview image')}
                  className='focus-visible:ring-ring size-14 cursor-zoom-in overflow-hidden rounded-lg focus-visible:ring-2'
                  onClick={() => setPreviewImage(image)}
                  type='button'
                >
                  <img
                    alt={t('Uploaded image')}
                    className='size-full object-cover'
                    draggable={false}
                    src={image}
                  />
                </button>
                <Button
                  aria-label={t('Remove image')}
                  className='absolute -top-1 -right-1 size-5 rounded-full'
                  disabled={props.disabled || isSubmitting}
                  onClick={() =>
                    setImages((prev) =>
                      prev.filter((_, itemIndex) => itemIndex !== index)
                    )
                  }
                  size='icon-sm'
                  type='button'
                  variant='secondary'
                >
                  <XIcon className='size-3' />
                </Button>
              </div>
            ))}
          </div>
        )}

        <Textarea
          autoComplete='off'
          className='max-h-52 min-h-12 resize-none border-0 bg-transparent px-4 py-3 shadow-none focus-visible:ring-0'
          disabled={props.disabled}
          onChange={(event) => setPrompt(event.target.value)}
          placeholder={t('Describe the image, or drop or paste images here...')}
          rows={hasPrompt ? 3 : 1}
          value={prompt}
        />

        <div className='flex flex-wrap items-center gap-2 px-3 pb-3'>
          <Button
            aria-label={t('Upload image')}
            disabled={
              images.length >= maxUploadImages ||
              props.disabled ||
              isSubmitting ||
              attachments.reading
            }
            onClick={() => fileInputRef.current?.click()}
            size='icon'
            type='button'
            variant='ghost'
          >
            <ImagePlusIcon className='size-4' />
          </Button>
          <input
            accept='image/*'
            className='hidden'
            multiple
            onChange={(event) => {
              void attachments.addFiles(Array.from(event.target.files || []))
              event.target.value = ''
            }}
            ref={fileInputRef}
            type='file'
          />

          <Select
            value={selectedAspectRatio}
            onValueChange={(value) => {
              if (value === 'original') {
                if (originalSize) setAspectRatio('original')
                return
              }
              const nextValue = getValidDrawingPreference(
                value,
                DRAWING_ASPECT_RATIO_VALUES
              )
              if (!nextValue) return
              setAspectRatio(nextValue)
              saveDrawingPreference(DRAWING_ASPECT_RATIO_STORAGE_KEY, nextValue)
            }}
          >
            <SelectTrigger aria-label={t('Aspect ratio')} size='sm'>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectGroup>
                {DRAWING_ASPECT_RATIOS.map((item) => (
                  <SelectItem key={item.value} value={item.value}>
                    {t(item.label)}
                  </SelectItem>
                ))}
                <SelectItem
                  disabled={!originalSize}
                  title={t('Original size is available with exactly one image')}
                  value='original'
                >
                  {t('Original size')}
                </SelectItem>
              </SelectGroup>
            </SelectContent>
          </Select>

          <Select
            disabled={usesOriginalSize}
            value={resolution}
            onValueChange={(value) => {
              const nextValue = getValidDrawingPreference(
                value,
                DRAWING_RESOLUTION_VALUES
              )
              if (!nextValue) return
              setResolution(nextValue)
              saveDrawingPreference(DRAWING_RESOLUTION_STORAGE_KEY, nextValue)
            }}
          >
            <SelectTrigger aria-label={t('Resolution')} size='sm'>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectGroup>
                {DRAWING_RESOLUTIONS.map((item) => (
                  <SelectItem key={item.value} value={item.value}>
                    {t(item.label)}
                  </SelectItem>
                ))}
              </SelectGroup>
            </SelectContent>
          </Select>

          {usesOriginalSize && originalSize && (
            <span className='text-muted-foreground text-xs'>
              {originalSize}
            </span>
          )}

          <div className='min-w-4 flex-1' />

          <BalancePopover balanceInfo={props.balanceInfo} />

          <Button
            aria-label={t('Send')}
            disabled={!canSubmit}
            onClick={() => setConfirmOpen(true)}
            size='icon'
            type='button'
          >
            {isSubmitting ? (
              <span className='size-4 animate-spin rounded-full border-2 border-current border-t-transparent' />
            ) : (
              <SendIcon className='size-4' />
            )}
          </Button>
        </div>
      </div>

      <Dialog
        open={Boolean(previewImage)}
        onOpenChange={(open) => {
          if (!open) setPreviewImage('')
        }}
      >
        <DialogContent className='max-h-[94dvh] sm:max-w-[min(96vw,1200px)]'>
          <DialogTitle>{t('Preview image')}</DialogTitle>
          <img
            alt={t('Preview image')}
            className='max-h-[80dvh] w-full rounded-md object-contain'
            src={previewImage || undefined}
          />
        </DialogContent>
      </Dialog>

      <AlertDialog open={confirmOpen} onOpenChange={setConfirmOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('Confirm submission')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('Submit the current prompt and start generating images?')}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={isSubmitting}>
              {t('Cancel')}
            </AlertDialogCancel>
            <AlertDialogAction
              disabled={!canSubmit}
              onClick={handleConfirmSubmit}
            >
              {t('Confirm')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}

function getStoredDrawingPreference<T extends string>(
  storageKey: string,
  values: readonly T[],
  fallback: T
): T {
  if (typeof window === 'undefined') return fallback

  try {
    const value = window.localStorage.getItem(storageKey)
    return getValidDrawingPreference(value, values) || fallback
  } catch {
    return fallback
  }
}

function getValidDrawingPreference<T extends string>(
  value: string | null,
  values: readonly T[]
): T | null {
  if (!value) return null
  return values.includes(value as T) ? (value as T) : null
}

function saveDrawingPreference(storageKey: string, value: string) {
  if (typeof window === 'undefined') return

  try {
    window.localStorage.setItem(storageKey, value)
  } catch {
    /* Ignore unavailable localStorage. */
  }
}
