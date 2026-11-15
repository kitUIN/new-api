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
import { useQuery } from '@tanstack/react-query'
import { useAuthStore } from '@/stores/auth-store'
import { getUserGroups, getUserModels } from '../api'
import {
  resolvePlaygroundGroup,
  resolvePlaygroundModel,
} from '../lib/selection'
import type { PlaygroundConfig } from '../types'

export function usePlaygroundOptions(config: PlaygroundConfig) {
  const userId = useAuthStore((state) => state.auth.user?.id)
  const groupsQuery = useQuery({
    queryKey: ['playground-groups', userId],
    queryFn: getUserGroups,
    refetchInterval: 60_000,
  })
  const groups = groupsQuery.data ?? []
  const group = resolvePlaygroundGroup(groups, config.group)
  const modelsQuery = useQuery({
    queryKey: ['playground-models', userId, group],
    queryFn: () => getUserModels(group),
    enabled: Boolean(group),
    refetchInterval: 60_000,
  })
  const models = modelsQuery.data ?? []
  const model = resolvePlaygroundModel(models, config.model)
  const hasError = groupsQuery.isError || modelsQuery.isError
  const isLoading =
    groupsQuery.isPending || Boolean(group && modelsQuery.isPending)

  return {
    groups,
    models,
    group,
    model,
    hasError,
    isLoading,
    canSend: Boolean(group && model) && !isLoading && !hasError,
    retry: () => {
      void groupsQuery.refetch()
      if (group) void modelsQuery.refetch()
    },
  }
}
