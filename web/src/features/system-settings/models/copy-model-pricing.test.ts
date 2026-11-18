import assert from 'node:assert/strict'
import { test } from 'node:test'
import {
  copyModelPricing,
  copyModelPricingSchema,
  type PricingMaps,
} from './copy-model-pricing'

test('copies each stored price and fallback without rewriting the source or its expression', () => {
  const maps: PricingMaps = {
    ModelPrice: { source: 0, other: 2 },
    ModelRatio: { source: 0.123456789 },
    CacheRatio: { source: 0 },
    CreateCacheRatio: { source: 1.25 },
    CompletionRatio: { source: 3 },
    ImageRatio: { source: 1 },
    AudioRatio: { source: 1.5 },
    AudioCompletionRatio: { source: 2.5 },
    'billing_setting.billing_mode': { source: 'tiered_expr' },
    'billing_setting.billing_expr': { source: '  tier("base", p)\n  * 2  ' },
  }
  const updates = copyModelPricing(maps, 'source', 'copy')
  assert.deepEqual(Object.keys(updates), Object.keys(maps))
  for (const [key, map] of Object.entries(maps)) {
    assert.deepEqual(JSON.parse(updates[key]), { ...map, copy: map.source })
    assert.equal(Object.hasOwn(map, 'copy'), false)
  }
})

test('only adds the new model to maps where the source has an entry', () => {
  const maps = { ModelPrice: { source: 0 }, ModelRatio: { other: 3 } }
  const updates = copyModelPricing(maps, 'source', 'copy')
  assert.deepEqual(Object.keys(updates), ['ModelPrice'])
  assert.deepEqual(JSON.parse(updates.ModelPrice), { source: 0, copy: 0 })
  assert.deepEqual(copyModelPricing(maps, 'missing', 'copy'), {})
})

test('requires a distinct nonblank name across all pricing maps', () => {
  const schema = copyModelPricingSchema({
    ModelPrice: { source: 0 },
    'billing_setting.billing_expr': { taken: '' },
  })
  for (const name of ['', '   ', 'source', ' taken ']) {
    assert.equal(schema.safeParse({ name }).success, false)
  }
  assert.deepEqual(schema.parse({ name: ' copy ' }), { name: 'copy' })
})
