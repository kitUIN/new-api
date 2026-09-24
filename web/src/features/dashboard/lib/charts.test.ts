import assert from 'node:assert/strict'
import { describe, test } from 'node:test'
import { processChartData } from './charts'
import type { QuotaDataItem } from '../types'

function getTooltipLabel(entry: {
  key: string | ((datum: Record<string, unknown>) => string)
}) {
  return typeof entry.key === 'function' ? entry.key({}) : entry.key
}

function getTooltipValue(
  entry: {
    value: string | number | ((datum: Record<string, unknown>) => string | number)
  },
  datum: Record<string, unknown>
) {
  return typeof entry.value === 'function' ? entry.value(datum) : entry.value
}

describe('processChartData', () => {
  test('shows model-first tooltip rows with cache write separated from output', () => {
    const data: QuotaDataItem[] = [
      {
        created_at: 1710000000,
        model_name: 'gpt-4o',
        quota: 123,
        count: 1,
        token_used: 190,
        prompt_tokens: 100,
        completion_tokens: 40,
        cache_read_tokens: 20,
        cache_write_tokens: 30,
      },
    ]

    const chartData = processChartData(data)
    const tooltip = chartData.spec_line.tooltip.mark.content as Array<{
      key: string | ((datum: Record<string, unknown>) => string)
      value: string | number | ((datum: Record<string, unknown>) => string | number)
    }>
    const labels = tooltip.map(getTooltipLabel)

    assert.deepEqual(labels, [
      'Model',
      'Total tokens',
      'Total input',
      'Cache Read',
      'Cache Write',
      'Output Tokens',
      'Total cost',
    ])

    const datum = {
      Tokens: 190,
      PromptTokens: 100,
      CacheReadTokens: 20,
      CompletionTokens: 40,
      CacheWriteTokens: 30,
      rawQuota: 123,
    }

    assert.equal(getTooltipValue(tooltip[0], { ...datum, Model: 'gpt-4o' }), 'gpt-4o')
    assert.equal(getTooltipValue(tooltip[4], datum), '30')
    assert.equal(getTooltipValue(tooltip[5], datum), '40')
  })
})
