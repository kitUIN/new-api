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
import type { ChangeEvent } from 'react'
import { ImagePlusIcon, SendIcon, XIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
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
import {
  Select,
  SelectContent,
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
import type { DrawingBalanceInfo, DrawingGenerateRequest } from '../types'
import { BalancePopover } from './balance-popover'

type AspectRatio = (typeof DRAWING_ASPECT_RATIOS)[number]['value']
type Resolution = (typeof DRAWING_RESOLUTIONS)[number]['value']

type DrawingInputBarProps = {
  balanceInfo: DrawingBalanceInfo
  disabled: boolean
  hasImage: boolean
  loading: boolean
  referenceImage: string
  onSubmit: (payload: DrawingGenerateRequest) => Promise<void>
}

export function DrawingInputBar(props: DrawingInputBarProps) {
  const { t } = useTranslation()
  const [prompt, setPrompt] = useState('')
  const [aspectRatio, setAspectRatio] = useState<AspectRatio>('1:1')
  const [resolution, setResolution] = useState<Resolution>('1K')
  const [images, setImages] = useState<string[]>([])
  const [confirmOpen, setConfirmOpen] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const fileInputRef = useRef<HTMLInputElement | null>(null)
  const hasPrompt = prompt.trim().length > 0
  const isSubmitting = props.loading || submitting
  const referenceImage = hasPrompt ? props.referenceImage : ''
  const maxUploadImages = Math.max(
    0,
    MAX_UPLOAD_IMAGES - (referenceImage ? 1 : 0)
  )

  const size = useMemo(
    () => resolveDrawingSize(aspectRatio, resolution),
    [aspectRatio, resolution]
  )

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

  const handleUpload = (event: ChangeEvent<HTMLInputElement>) => {
    const files = Array.from(event.target.files || [])
    if (files.length === 0) return

    if (images.length + files.length > maxUploadImages) {
      toast.warning(
        t('You can upload up to {{count}} images', { count: maxUploadImages })
      )
      event.target.value = ''
      return
    }

    for (const file of files) {
      const reader = new FileReader()
      reader.onload = () => {
        const result = typeof reader.result === 'string' ? reader.result : ''
        if (result) setImages((prev) => [...prev, result])
      }
      reader.readAsDataURL(file)
    }
    event.target.value = ''
  }

  const handleConfirmSubmit = async () => {
    if (!payload.prompt || props.disabled || isSubmitting) return
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

      <div className='bg-background ring-border overflow-hidden rounded-xl ring-1'>
        {(referenceImage || images.length > 0) && (
          <div className='flex flex-wrap gap-2 px-4 pt-3'>
            {referenceImage && (
              <img
                alt={t('Reference image')}
                className='border-primary size-14 rounded-lg border-2 object-cover'
                src={referenceImage}
              />
            )}
            {images.map((image, index) => (
              <div
                className='relative size-14'
                key={`${image.slice(0, 24)}-${index}`}
              >
                <img
                  alt={t('Uploaded image')}
                  className='size-14 rounded-lg object-cover'
                  src={image}
                />
                <Button
                  aria-label={t('Remove image')}
                  className='absolute -top-1 -right-1 size-5 rounded-full'
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
          placeholder={t('Describe the image you want to generate...')}
          rows={hasPrompt ? 3 : 1}
          value={prompt}
        />

        <div className='flex flex-wrap items-center gap-2 px-3 pb-3'>
          <Button
            aria-label={t('Upload image')}
            disabled={
              images.length >= maxUploadImages || props.disabled || isSubmitting
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
            onChange={handleUpload}
            ref={fileInputRef}
            type='file'
          />

          <Select
            value={aspectRatio}
            onValueChange={(value) =>
              value && setAspectRatio(value as AspectRatio)
            }
          >
            <SelectTrigger size='sm'>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {DRAWING_ASPECT_RATIOS.map((item) => (
                <SelectItem key={item.value} value={item.value}>
                  {t(item.label)}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>

          <Select
            value={resolution}
            onValueChange={(value) =>
              value && setResolution(value as Resolution)
            }
          >
            <SelectTrigger size='sm'>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {DRAWING_RESOLUTIONS.map((item) => (
                <SelectItem key={item.value} value={item.value}>
                  {t(item.label)}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>

          <div className='min-w-4 flex-1' />

          <BalancePopover balanceInfo={props.balanceInfo} />

          <Button
            aria-label={t('Send')}
            disabled={!hasPrompt || props.disabled || isSubmitting}
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
              disabled={isSubmitting}
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
