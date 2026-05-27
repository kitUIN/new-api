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
import { formatCurrencyFromUSD } from '@/lib/currency'
import type { PricingModel } from '@/features/pricing/types'
import { DEFAULT_DRAWING_MODEL } from '../constants'
import type { DrawingBalanceInfo } from '../types'

type BalanceInput = {
  userQuota?: number
  quotaPerUnit?: number
  model?: PricingModel | null
  groupRatio?: Record<string, number>
  pricingLoading: boolean
}

export function buildDrawingBalanceInfo(
  input: BalanceInput
): DrawingBalanceInfo {
  const balanceUSD = quotaToUsdAmount(input.userQuota, input.quotaPerUnit)
  const tone = getBalanceTone(balanceUSD)
  const usedGroup = resolvePricingGroup(input.model, input.groupRatio || {})
  const unitPriceUSD = getUnitPriceUSD(
    input.model,
    usedGroup,
    input.groupRatio || {}
  )

  return {
    balanceText: formatCurrencyFromUSD(balanceUSD, {
      digitsLarge: 2,
      digitsSmall: 4,
      abbreviate: false,
    }),
    balanceUSD,
    availableGenerationsText: getAvailableGenerations(balanceUSD, unitPriceUSD),
    modelName: DEFAULT_DRAWING_MODEL,
    priceText:
      unitPriceUSD > 0
        ? formatCurrencyFromUSD(unitPriceUSD, {
            digitsLarge: 4,
            digitsSmall: 6,
            abbreviate: false,
          })
        : '',
    priceUnavailable:
      !input.model && !input.pricingLoading ? 'Model pricing not found' : '',
    pricingLoading: input.pricingLoading,
    tone,
    usedGroup,
  }
}

function quotaToUsdAmount(quota = 0, quotaPerUnit = 1): number {
  const safeQuotaPerUnit =
    Number.isFinite(quotaPerUnit) && quotaPerUnit > 0 ? quotaPerUnit : 1
  return Number(quota || 0) / safeQuotaPerUnit
}

function getBalanceTone(balanceUSD: number): DrawingBalanceInfo['tone'] {
  if (balanceUSD < 0.1) return 'danger'
  if (balanceUSD < 1) return 'warning'
  return 'success'
}

function resolvePricingGroup(
  model: PricingModel | null | undefined,
  groupRatio: Record<string, number>
): string {
  const enableGroups = Array.isArray(model?.enable_groups)
    ? model.enable_groups
    : []
  if (
    enableGroups.includes('gpt-image') &&
    groupRatio['gpt-image'] !== undefined
  ) {
    return 'gpt-image'
  }
  return enableGroups[0] || 'all'
}

function getUnitPriceUSD(
  model: PricingModel | null | undefined,
  group: string,
  groupRatio: Record<string, number>
): number {
  if (!model || model.quota_type !== 1) return 0
  const ratio = Number(groupRatio[group] ?? 1)
  const price = Number(model.model_price || 0) * ratio
  return Number.isFinite(price) ? price : 0
}

function getAvailableGenerations(balanceUSD: number, unitPriceUSD: number) {
  if (!Number.isFinite(unitPriceUSD) || unitPriceUSD <= 0) return ''
  return String(Math.max(0, Math.floor(balanceUSD / unitPriceUSD)))
}
