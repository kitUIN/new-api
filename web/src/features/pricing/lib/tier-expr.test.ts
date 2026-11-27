import assert from 'node:assert/strict'
import { describe, test } from 'node:test'
import {
  createLongContextDoubleConfig,
  evalExprLocally,
  generateExprFromVisualConfig,
  tryParseVisualConfig,
} from './tier-expr'

describe('generateExprFromVisualConfig', () => {
  test('uses an unconditional tier as fallback even when it appears first', () => {
    const expr = generateExprFromVisualConfig({
      tiers: [
        {
          label: 'base',
          conditions: [],
          input_unit_cost: 5,
          output_unit_cost: 30,
          cache_mode: 'generic',
          cache_read_unit_cost: 0.5,
          cache_create_unit_cost: 3,
        },
        {
          label: 'no_cache',
          conditions: [{ var: 'group', op: '==', value: 'vip' }],
          input_unit_cost: 5,
          output_unit_cost: 30,
          cache_mode: 'generic',
          cache_read_unit_cost: 5,
          cache_create_unit_cost: 30,
        },
      ],
    })

    assert.equal(
      expr,
      'group == "vip" ? tier("no_cache", p * 5 + c * 30 + cr * 5 + cc * 30) : tier("base", p * 5 + c * 30 + cr * 0.5 + cc * 3)'
    )
  })
})

test('long context template doubles each unit price only above the full input threshold', () => {
  const config = createLongContextDoubleConfig({
    input_unit_cost: 3,
    output_unit_cost: 15,
    cache_read_unit_cost: 0.3,
    cache_create_unit_cost: 3.75,
    cache_create_1h_unit_cost: 6,
    audio_output_unit_cost: 9,
  })
  const expr = generateExprFromVisualConfig(config)
  assert.deepEqual(tryParseVisualConfig(expr), config)
  const extra = {
    cacheReadTokens: 10000,
    cacheCreateTokens: 0,
    cacheCreate1hTokens: 0,
    imageTokens: 0,
    audioInputTokens: 0,
    audioOutputTokens: 20,
  }
  const promptAtThreshold = 272_000 - extra.cacheReadTokens
  const standard = evalExprLocally(expr, promptAtThreshold, 10, extra)
  assert.equal(standard.matchedTier, 'standard')
  const long = evalExprLocally(expr, promptAtThreshold + 1, 10, extra)
  assert.equal(long.matchedTier, 'long_context')
  assert.equal(long.cost, 2 * (standard.cost + 3))
})
