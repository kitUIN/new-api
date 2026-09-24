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
import { PublicLayout } from '@/components/layout'
import { PageTransition } from '@/components/page-transition'
import { GroupHealthGrid } from '@/features/dashboard/components/groups/group-health-grid'

export function GroupHealth() {
  const { t } = useTranslation()

  return (
    <PublicLayout showMainContainer={false}>
      <PageTransition className='mx-auto w-full max-w-[1280px] px-3 pt-20 pb-10 sm:px-6 sm:pt-24 sm:pb-12 xl:px-8'>
        <div className='mb-5 flex flex-col gap-1'>
          <h1 className='text-2xl font-semibold tracking-tight'>
            {t('Group Health')}
          </h1>
          <p className='text-muted-foreground text-sm'>
            {t('TTFT percentiles, throughput, and 30-day uptime by group')}
          </p>
        </div>
        <GroupHealthGrid />
      </PageTransition>
    </PublicLayout>
  )
}
