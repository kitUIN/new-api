import type { TFunction } from 'i18next'

const RULE_AUTO_GROUP_NAMES = new Set([
  '自动分组:codex-low',
  '自动分组:codex-pro',
  '自动分组:kiro',
  '自动分组:gemini',
])

const LEGACY_RULE_AUTO_GROUP_NAMES: Record<string, string> = {
  'auto:codex-low': '自动分组:codex-low',
  'auto:codex-pro': '自动分组:codex-pro',
  'auto:kiro': '自动分组:kiro',
  'auto:gemini': '自动分组:gemini',
}

export function normalizeRuleAutoGroupName(group: string): string | null {
  if (RULE_AUTO_GROUP_NAMES.has(group)) return group
  return LEGACY_RULE_AUTO_GROUP_NAMES[group] ?? null
}

export function isRuleAutoGroupName(group: string): boolean {
  return normalizeRuleAutoGroupName(group) !== null
}

export function getRuleAutoGroupLabel(
  group: string,
  fallback: string,
  _t: TFunction
): string {
  return normalizeRuleAutoGroupName(group) ?? fallback
}
