import type { TFunction } from 'i18next'

const RULE_AUTO_GROUP_LABEL_KEYS: Record<string, string> = {
  'auto:codex-low': 'Codex low-cost group',
  'auto:codex-pro': 'Codex Pro group',
  'auto:kiro': 'Kiro group',
  'auto:gemini': 'Gemini group',
}

export function getRuleAutoGroupLabel(
  group: string,
  fallback: string,
  t: TFunction
): string {
  const key = RULE_AUTO_GROUP_LABEL_KEYS[group]
  return key ? t(key) : fallback
}
