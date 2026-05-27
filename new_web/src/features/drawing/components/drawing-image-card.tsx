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
import { useState } from 'react'
import { DownloadIcon, FileTextIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'
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
}

export function DrawingImageCard(props: DrawingImageCardProps) {
  const { t } = useTranslation()
  const [previewOpen, setPreviewOpen] = useState(false)
  const [promptOpen, setPromptOpen] = useState(false)
  const imageSrc = getDrawingImageSource(props.image)

  if (!imageSrc) return null

  const handleDownload = () => {
    const link = document.createElement('a')
    link.href = imageSrc
    link.download = `generated-${Date.now()}.png`
    link.rel = 'noreferrer'
    link.click()
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
            alt={props.image.revised_prompt || t('Generated image')}
            className='block max-h-[68dvh] w-full object-contain'
            src={imageSrc}
          />
          <span className='pointer-events-none absolute inset-0 bg-black/0 transition group-hover:bg-black/5' />
        </button>

        <div className='mt-2 flex justify-center gap-1'>
          {props.image.revised_prompt && (
            <Button
              aria-label={t('View prompt')}
              onClick={() => setPromptOpen(true)}
              size='icon-sm'
              type='button'
              variant='ghost'
            >
              <FileTextIcon className='size-4' />
            </Button>
          )}
          <Button
            aria-label={t('Download')}
            onClick={handleDownload}
            size='icon-sm'
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
            alt={props.image.revised_prompt || t('Generated image')}
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
          <p className='text-muted-foreground max-h-[60dvh] overflow-auto text-sm leading-relaxed'>
            {props.image.revised_prompt}
          </p>
        </DialogContent>
      </Dialog>
    </>
  )
}
