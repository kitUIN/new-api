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
import { useTranslation } from 'react-i18next'
import type { RequestClientConfig } from '@/features/usage-logs/lib/request-client'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'
import { RequestClientEditor } from './request-client-editor'

export function RequestClientSettingsSection(props: { value: string }) {
  const { t } = useTranslation()
  const update = useUpdateOption()
  let clients: RequestClientConfig[]
  try {
    clients = JSON.parse(props.value) as RequestClientConfig[]
    if (!Array.isArray(clients)) throw new Error('Invalid configuration')
  } catch {
    return <p role='alert'>{t('客户端配置格式错误，请检查已保存的配置')}</p>
  }
  return (
    <SettingsSection title={t('客户端识别')}>
      <RequestClientEditor
        clients={clients}
        onSave={async (value) => {
          const result = await update.mutateAsync({
            key: 'RequestClientRules',
            value: JSON.stringify(value),
          })
          if (!result.success)
            throw new Error(result.message || t('Failed to update setting'))
        }}
      />
    </SettingsSection>
  )
}
