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
import { Box, Boxes } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'

type GroupTypeIconProps = {
  isCombinationGroup?: boolean
  isAutoGroup?: boolean
  combinationMembers?: { group: string; ratio: number }[]
}

export function GroupTypeIcon(props: GroupTypeIconProps) {
  const { t } = useTranslation()
  let label = t('普通分组')
  if (props.isAutoGroup) label = t('Group')
  if (props.isCombinationGroup) label = t('组合分组')
  const Icon = props.isCombinationGroup ? Boxes : Box

  return (
    <TooltipProvider delay={300}>
      <Tooltip>
        <TooltipTrigger
          render={
            <span
              className={cn(
                'inline-flex size-5 shrink-0 items-center justify-center',
                props.isCombinationGroup
                  ? 'text-primary'
                  : 'text-muted-foreground'
              )}
              role='img'
              aria-label={label}
            >
              <Icon className='size-4' aria-hidden='true' />
            </span>
          }
        />
        <TooltipContent>
          <div className='flex min-w-0 flex-col gap-1.5'>
            <span>{label}</span>
            {props.isCombinationGroup && !!props.combinationMembers?.length && (
              <dl className='flex flex-col gap-1'>
                {props.combinationMembers.map((member) => (
                  <div
                    key={member.group}
                    className='flex items-start justify-between gap-4'
                  >
                    <dt className='min-w-0 break-all'>{member.group}</dt>
                    <dd className='shrink-0 font-mono tabular-nums'>
                      {member.ratio}x
                    </dd>
                  </div>
                ))}
              </dl>
            )}
          </div>
        </TooltipContent>
      </Tooltip>
    </TooltipProvider>
  )
}
