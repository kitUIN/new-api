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
import { useMemo, useState } from 'react'
import { CopyIcon, DownloadIcon, FileTextIcon, ImageIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { getDrawingImageSource } from '../lib/images'
import type { DrawingImageResult } from '../types'

type DrawingImageCardProps = {
  image: DrawingImageResult
  inputImages?: string[]
}

export function DrawingImageCard(props: DrawingImageCardProps) {
  const { t } = useTranslation()
  const { copyToClipboard } = useCopyToClipboard()
  const [previewOpen, setPreviewOpen] = useState(false)
  const [promptOpen, setPromptOpen] = useState(false)
  const [sourceImagesOpen, setSourceImagesOpen] = useState(false)
  const imageSrc = getDrawingImageSource(props.image)
  const inputImages = useMemo(
    () => Array.from(new Set(props.inputImages || [])),
    [props.inputImages]
  )
  const revisedPrompt = props.image.revised_prompt || ''

  if (!imageSrc) return null

  const handleDownload = (source = imageSrc, filenamePrefix = 'generated') => {
    const link = document.createElement('a')
    link.href = source
    link.download = `${filenamePrefix}-${Date.now()}.png`
    link.rel = 'noreferrer'
    link.click()
  }

  const handleCopyPrompt = () => {
    if (revisedPrompt) void copyToClipboard(revisedPrompt)
  }

  return (
    <>
      <div className='w-full min-w-0'>
        <button
          type='button'
          className='group bg-muted/30 ring-border hover:ring-foreground/30 relative block w-full overflow-hidden rounded-lg ring-1 transition'
          onClick={() => setPreviewOpen(true)}
        >
          <img
            alt={revisedPrompt || t('Generated image')}
            className='block max-h-[68dvh] w-full object-contain'
            src={imageSrc}
          />
          <span className='pointer-events-none absolute inset-0 bg-black/0 transition group-hover:bg-black/5' />
        </button>

        <div className='mt-2 flex justify-center gap-1'>
          {revisedPrompt && (
            <Button
              aria-label={t('View revised prompt')}
              onClick={() => setPromptOpen(true)}
              size='icon-sm'
              title={t('View revised prompt')}
              type='button'
              variant='ghost'
            >
              <FileTextIcon className='size-4' />
            </Button>
          )}
          {inputImages.length > 0 && (
            <Button
              aria-label={t('View original images')}
              onClick={() => setSourceImagesOpen(true)}
              size='icon-sm'
              title={t('View original images')}
              type='button'
              variant='ghost'
            >
              <ImageIcon className='size-4' />
            </Button>
          )}
          <Button
            aria-label={t('Download')}
            onClick={() => handleDownload()}
            size='icon-sm'
            title={t('Download')}
            type='button'
            variant='ghost'
          >
            <DownloadIcon className='size-4' />
          </Button>
        </div>
      </div>

      <Dialog open={previewOpen} onOpenChange={setPreviewOpen}>
        <DialogContent className='max-h-[94dvh] max-w-[min(96vw,1200px)] p-3'>
          <img
            alt={revisedPrompt || t('Generated image')}
            className='max-h-[88dvh] w-full rounded-md object-contain'
            src={imageSrc}
          />
        </DialogContent>
      </Dialog>

      <Dialog open={promptOpen} onOpenChange={setPromptOpen}>
        <DialogContent className='max-w-lg'>
          <DialogHeader>
            <DialogTitle>{t('Revised Prompt')}</DialogTitle>
          </DialogHeader>
          <div className='mb-2 flex justify-end'>
            <Button
              aria-label={t('Copy revised prompt')}
              onClick={handleCopyPrompt}
              size='icon-sm'
              title={t('Copy revised prompt')}
              type='button'
              variant='ghost'
            >
              <CopyIcon className='size-4' />
            </Button>
          </div>
          <p className='text-muted-foreground max-h-[60dvh] overflow-auto rounded-md border px-3 py-2 text-sm leading-relaxed whitespace-pre-wrap'>
            {revisedPrompt}
          </p>
        </DialogContent>
      </Dialog>

      <Dialog open={sourceImagesOpen} onOpenChange={setSourceImagesOpen}>
        <DialogContent className='max-h-[92dvh] max-w-[min(94vw,720px)] overflow-hidden'>
          <DialogHeader>
            <DialogTitle>{t('Original Images')}</DialogTitle>
          </DialogHeader>
          <div className='grid max-h-[72dvh] grid-cols-2 gap-2 overflow-auto pr-1'>
            {inputImages.map((source, index) => (
              <div
                className='group bg-muted/30 relative overflow-hidden rounded-md border'
                key={`${source.slice(0, 32)}-${index}`}
              >
                <img
                  alt={t('Original image')}
                  className='aspect-square w-full object-cover'
                  src={source}
                />
                <Button
                  aria-label={t('Download original image')}
                  className='bg-background/90 absolute right-1.5 bottom-1.5 shadow-sm transition sm:opacity-0 sm:group-hover:opacity-100 sm:focus-visible:opacity-100'
                  onClick={() =>
                    handleDownload(source, `original-${index + 1}`)
                  }
                  size='icon-sm'
                  title={t('Download original image')}
                  type='button'
                  variant='secondary'
                >
                  <DownloadIcon className='size-4' />
                </Button>
              </div>
            ))}
          </div>
        </DialogContent>
      </Dialog>
    </>
  )
}
