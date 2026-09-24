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
import { CircleDollarSignIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover'
import type { DrawingBalanceInfo } from '../types'

type BalancePopoverProps = {
  balanceInfo: DrawingBalanceInfo
}

const toneClassName: Record<DrawingBalanceInfo['tone'], string> = {
  success: 'border-emerald-500 text-emerald-600',
  warning: 'border-amber-500 text-amber-600',
  danger: 'border-destructive text-destructive',
}

export function BalancePopover(props: BalancePopoverProps) {
  const { t } = useTranslation()

  return (
    <Popover>
      <PopoverTrigger
        render={
          <Button
            aria-label={t('Current balance')}
            size='icon'
            type='button'
            variant='ghost'
          />
        }
      >
        <span
          className={cn(
            'flex size-5 items-center justify-center rounded-full border-2',
            toneClassName[props.balanceInfo.tone]
          )}
        >
          <CircleDollarSignIcon className='size-3.5' />
        </span>
      </PopoverTrigger>
      <PopoverContent align='end' className='w-72'>
        <div className='space-y-3 text-sm'>
          <div className='flex items-center justify-between gap-3'>
            <span className='text-muted-foreground'>
              {t('Current balance')}
            </span>
            <span
              className={cn(
                'font-medium',
                toneClassName[props.balanceInfo.tone]
              )}
            >
              {props.balanceInfo.balanceText}
            </span>
          </div>

          <div className='border-t pt-3'>
            <div className='font-medium'>{props.balanceInfo.modelName}</div>
            <div className='mt-2 space-y-1 text-xs'>
              <div className='flex justify-between gap-3'>
                <span className='text-muted-foreground'>{t('Price')}</span>
                <span>
                  {props.balanceInfo.pricingLoading
                    ? t('Loading...')
                    : props.balanceInfo.priceText ||
                      t(props.balanceInfo.priceUnavailable || 'Unavailable')}
                </span>
              </div>
              {props.balanceInfo.availableGenerationsText && (
                <div className='flex justify-between gap-3'>
                  <span className='text-muted-foreground'>
                    {t('Estimated available generations')}
                  </span>
                  <span>{props.balanceInfo.availableGenerationsText}</span>
                </div>
              )}
              <div className='flex justify-between gap-3'>
                <span className='text-muted-foreground'>{t('Group')}</span>
                <span>{props.balanceInfo.usedGroup}</span>
              </div>
            </div>
          </div>

          <p className='text-muted-foreground text-xs'>
            {t(
              'For reference only. Actual billing is based on final settlement.'
            )}
          </p>
        </div>
      </PopoverContent>
    </Popover>
  )
}
