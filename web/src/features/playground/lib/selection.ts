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
import { DEFAULT_GROUP } from '../constants'
import type { GroupOption, ModelOption } from '../types'

export function resolvePlaygroundGroup(
  groups: GroupOption[],
  selected: string
): string {
  if (groups.some((group) => group.value === selected)) return selected
  return (
    groups.find((group) => group.value === DEFAULT_GROUP)?.value ??
    groups[0]?.value ??
    ''
  )
}

export function resolvePlaygroundModel(
  models: ModelOption[],
  selected: string
): string {
  if (models.some((model) => model.value === selected)) return selected
  return models[0]?.value ?? ''
}
