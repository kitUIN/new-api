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
import { FILTER_ALL } from '../constants'
import type { PricingModel } from '../types'

export function getGroupPriceMultiplier(
  model: PricingModel,
  group: string,
  groupRatio: number
): number {
  return groupRatio * (model.subscription_model_ratios?.[group] ?? 1)
}

export function getDisplayGroupRatio(
  model: PricingModel,
  selectedGroup?: string
): number {
  const groups = Array.isArray(model.enable_groups) ? model.enable_groups : []
  const ratios = model.group_ratio || {}

  if (
    selectedGroup &&
    selectedGroup !== FILTER_ALL &&
    groups.includes(selectedGroup)
  ) {
    return getGroupPriceMultiplier(
      model,
      selectedGroup,
      ratios[selectedGroup] ?? 1
    )
  }

  let minRatio = Number.POSITIVE_INFINITY
  for (const group of groups) {
    if (
      ratios[group] !== undefined ||
      model.subscription_model_ratios?.[group] !== undefined
    ) {
      minRatio = Math.min(
        minRatio,
        getGroupPriceMultiplier(model, group, ratios[group] ?? 1)
      )
    }
  }

  return minRatio === Number.POSITIVE_INFINITY ? 1 : minRatio
}
