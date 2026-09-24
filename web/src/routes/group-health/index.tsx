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
import z from 'zod'
import { createFileRoute, redirect } from '@tanstack/react-router'
import { useAuthStore } from '@/stores/auth-store'
import { getFreshModuleAccess } from '@/lib/nav-modules'
import { GroupHealth } from '@/features/group-health'

const groupHealthSearchSchema = z.object({
  access_token: z.string().optional(),
})

export const Route = createFileRoute('/group-health/')({
  validateSearch: groupHealthSearchSchema,
  beforeLoad: async ({ location, search }) => {
    const access = await getFreshModuleAccess('groupHealth')
    if (!access.enabled) {
      throw redirect({ to: '/' })
    }
    if (!access.requireAuth) {
      return
    }

    const { auth } = useAuthStore.getState()
    if (auth.user) {
      return
    }

    const configuredToken = access.accessToken?.trim()
    const providedToken = search.access_token?.trim()
    if (configuredToken && providedToken === configuredToken) {
      return
    }

    throw redirect({
      to: '/sign-in',
      search: { redirect: location.href },
    })
  },
  component: GroupHealth,
})
