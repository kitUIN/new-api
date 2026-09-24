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
import { ExternalLink, KeyRound } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { FaQq } from 'react-icons/fa'
import { toast } from 'sonner'
import { CopyButton } from '@/components/copy-button'
import { Alert, AlertDescription } from '@/components/ui/alert'
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
import { bindQQ, type QQBindingSession } from '../../api'

interface QQBindDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  session: QQBindingSession | null
  friendLink?: string
  userId?: number
  onSuccess: () => void
}

export function QQBindDialog(props: QQBindDialogProps) {
  const { t } = useTranslation()
  const [code, setCode] = useState('')
  const [isSubmitting, setIsSubmitting] = useState(false)

  const command = useMemo(() => {
    if (props.session?.command) return props.session.command
    return `/nachoai b ${props.userId ?? ''}`.trim()
  }, [props.session?.command, props.userId])

  const handleSubmit = async () => {
    if (!code.trim()) {
      toast.error(t('Enter the QQ verification code'))
      return
    }

    setIsSubmitting(true)
    try {
      const res = await bindQQ(code.trim())
      if (res.success) {
        toast.success(t('QQ account bound successfully'))
        setCode('')
        props.onOpenChange(false)
        props.onSuccess()
      } else {
        toast.error(res.message || t('Binding failed'))
      }
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('Binding failed'))
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <Dialog
      open={props.open}
      onOpenChange={(open) => {
        if (!open) setCode('')
        props.onOpenChange(open)
      }}
    >
      <DialogContent className='sm:max-w-md'>
        <DialogHeader>
          <DialogTitle className='flex items-center gap-2'>
            <FaQq className='h-4 w-4' />
            {t('Bind QQ Account')}
          </DialogTitle>
          <DialogDescription>
            {t('Add the QQ account below and send the binding command.')}
          </DialogDescription>
        </DialogHeader>

        <div className='space-y-4 py-2'>
          <Alert>
            <FaQq className='h-4 w-4' />
            <AlertDescription className='flex min-w-0 items-center gap-1.5'>
              <span className='shrink-0'>{t('Add QQ friend')}:</span>
              <span className='min-w-0 truncate'>
                {props.session?.qq_number || t('Admin QQ')}
              </span>
              {props.friendLink ? (
                <Button
                  type='button'
                  variant='ghost'
                  size='icon-xs'
                  className='-my-1'
                  onClick={() =>
                    window.open(props.friendLink, '_blank', 'noopener')
                  }
                  aria-label={t('Open QQ friend link')}
                >
                  <ExternalLink className='h-3.5 w-3.5' />
                </Button>
              ) : null}
            </AlertDescription>
          </Alert>

          <div className='space-y-2'>
            <p className='text-sm font-medium'>{t('Send this command')}</p>
            <div className='bg-muted flex min-w-0 items-center gap-2 rounded-lg border px-2.5 py-2'>
              <code className='min-w-0 flex-1 truncate font-mono text-sm'>
                {command}
              </code>
              <CopyButton
                value={command}
                tooltip={t('Copy binding command')}
                aria-label={t('Copy binding command')}
              />
            </div>
          </div>

          <div className='space-y-2'>
            <label className='text-sm font-medium' htmlFor='qq-bind-code'>
              {t('QQ returned verification code')}
            </label>
            <div className='relative'>
              <KeyRound className='text-muted-foreground pointer-events-none absolute top-1/2 left-2.5 h-4 w-4 -translate-y-1/2' />
              <Input
                id='qq-bind-code'
                value={code}
                onChange={(event) => setCode(event.target.value)}
                placeholder={t('Enter the QQ verification code')}
                className='pl-8'
                autoComplete='one-time-code'
                onKeyDown={(event) => {
                  if (event.key === 'Enter') {
                    event.preventDefault()
                    handleSubmit()
                  }
                }}
              />
            </div>
          </div>
        </div>

        <DialogFooter>
          <Button
            type='button'
            variant='outline'
            onClick={() => props.onOpenChange(false)}
          >
            {t('Cancel')}
          </Button>
          <Button type='button' onClick={handleSubmit} disabled={isSubmitting}>
            <FaQq className='h-4 w-4' />
            {isSubmitting ? t('Binding...') : t('Bind')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
