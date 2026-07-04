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
import { useEffect, useState } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { Loader2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { updateChannelProvider } from '../../api'
import { channelsQueryKeys } from '../../lib'
import { useChannels } from '../channels-provider'

type EditProviderDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function EditProviderDialog({
  open,
  onOpenChange,
}: EditProviderDialogProps) {
  const { t } = useTranslation()
  const { currentProvider } = useChannels()
  const queryClient = useQueryClient()

  const [name, setName] = useState('')
  const [baseUrl, setBaseUrl] = useState('')
  const [remark, setRemark] = useState('')
  const [isSubmitting, setIsSubmitting] = useState(false)

  useEffect(() => {
    if (open && currentProvider) {
      setName(currentProvider.name || '')
      setBaseUrl(currentProvider.base_url || '')
      setRemark('')
    }
  }, [open, currentProvider])

  const handleSubmit = async () => {
    if (!currentProvider) return
    if (!baseUrl.trim()) {
      toast.error(t('Base URL is required'))
      return
    }

    setIsSubmitting(true)
    try {
      const res = await updateChannelProvider({
        id: currentProvider.id,
        provider_id: currentProvider.provider_id,
        name: name.trim(),
        base_url: baseUrl.trim(),
        ...(remark.trim() ? { remark: remark.trim() } : {}),
      })
      if (res.success) {
        toast.success(t('Provider updated successfully'))
        queryClient.invalidateQueries({ queryKey: channelsQueryKeys.lists() })
        onOpenChange(false)
      } else {
        toast.error(res.message || t('Failed to update provider'))
      }
    } catch (err: unknown) {
      toast.error(
        err instanceof Error ? err.message : t('Failed to update provider')
      )
    } finally {
      setIsSubmitting(false)
    }
  }

  if (!currentProvider) return null

  return (
    <Dialog open={open} onOpenChange={(v) => !v && onOpenChange(false)}>
      <DialogContent className='max-w-md'>
        <DialogHeader>
          <DialogTitle>{t('Edit Provider')}</DialogTitle>
          <DialogDescription>
            {t('Modify the provider name and base URL.')}
          </DialogDescription>
        </DialogHeader>

        <div className='space-y-4 py-2'>
          <div className='space-y-2'>
            <Label htmlFor='provider-name'>{t('Name')}</Label>
            <Input
              id='provider-name'
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder={t('Provider name')}
            />
          </div>

          <div className='space-y-2'>
            <Label htmlFor='provider-base-url'>
              {t('Base URL')}
              <span className='text-destructive ml-1'>*</span>
            </Label>
            <Input
              id='provider-base-url'
              value={baseUrl}
              onChange={(e) => setBaseUrl(e.target.value)}
              placeholder='https://api.example.com'
              className='font-mono text-sm'
            />
          </div>

          <div className='space-y-2'>
            <Label htmlFor='provider-remark'>
              {t('Remark')}
              <span className='text-muted-foreground ml-2 text-xs'>
                {t('(optional)')}
              </span>
            </Label>
            <Input
              id='provider-remark'
              value={remark}
              onChange={(e) => setRemark(e.target.value)}
              placeholder={t('Leave empty to keep current remark')}
            />
          </div>
        </div>

        <DialogFooter>
          <Button variant='outline' onClick={() => onOpenChange(false)}>
            {t('Cancel')}
          </Button>
          <Button onClick={handleSubmit} disabled={isSubmitting}>
            {isSubmitting && <Loader2 className='mr-2 h-4 w-4 animate-spin' />}
            {t('Save')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
