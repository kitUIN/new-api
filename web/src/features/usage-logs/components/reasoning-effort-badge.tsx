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
import '@/styles/reasoning-effort.css'
import { useTranslation } from 'react-i18next'
import { StatusBadge, type StatusVariant } from '@/components/status-badge'

function getReasoningEffortVariant(effort: string): StatusVariant {
  switch (effort) {
    case 'none':
      return 'grey'
    case 'minimal':
      return 'teal'
    case 'low':
      return 'green'
    case 'medium':
      return 'blue'
    case 'high':
      return 'orange'
    case 'xhigh':
      return 'red'
    default:
      return 'violet'
  }
}

export function ReasoningEffortBadge(props: { effort: string }) {
  const { t } = useTranslation()
  const effort = props.effort.trim().toLowerCase()

  if (effort === 'ultra') {
    return (
      <StatusBadge
        label={t('ultra')}
        className='reasoning-effort-ultra'
        size='sm'
        copyable={false}
      />
    )
  }

  return (
    <StatusBadge
      label={t(props.effort)}
      variant={getReasoningEffortVariant(effort)}
      size='sm'
      copyable={false}
    />
  )
}
