import assert from 'node:assert/strict'
import { describe, test } from 'node:test'
import { generateExprFromVisualConfig } from './tier-expr'

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
