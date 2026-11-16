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
import { useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { useNavigationConfirmationStore } from '@/stores/navigation-confirmation-store'
import {
  installLinkConfirmation,
  opensNewContext,
} from '@/lib/confirmed-navigation'
import { ConfirmDialog } from '@/components/confirm-dialog'

export function NavigationConfirmationDialog() {
  const { t } = useTranslation()
  const pending = useNavigationConfirmationStore((state) => state.pending)
  const confirm = useNavigationConfirmationStore((state) => state.confirm)
  const cancel = useNavigationConfirmationStore((state) => state.cancel)

  useEffect(installLinkConfirmation, [])

  return (
    <ConfirmDialog
      open={pending !== null}
      onOpenChange={(open) => {
        if (!open) cancel()
      }}
      title={t('navigation.confirm.title')}
      desc={
        <div className='min-w-0 space-y-3'>
          <p>
            {pending && opensNewContext(pending.target)
              ? t('navigation.confirm.newTab')
              : t('navigation.confirm.leavePage')}
          </p>
          <p
            className='bg-muted max-h-40 overflow-auto rounded-md p-3 text-left font-mono text-xs break-all'
            dir='ltr'
          >
            {pending?.url}
          </p>
        </div>
      }
      confirmText={t('Continue')}
      handleConfirm={confirm}
    />
  )
}
