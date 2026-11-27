import assert from 'node:assert/strict'
import { test } from 'node:test'
import {
  getModelPriceMultiplier,
  getFastModeLabel,
  getServiceTierBillingMultiplier,
  getServiceTierLabel,
} from './format'

test('ultrafast has its own label and an 8x fallback price', () => {
  const other = { service_tier: 'ultrafast' }
  assert.equal(getServiceTierLabel(other), 'ultrafast')
  assert.equal(getFastModeLabel(other, (key) => key), 'Ultrafast')
  assert.equal(getServiceTierBillingMultiplier(other), 8)
})

test('log prices use the recorded multiplier ahead of tier defaults', () => {
  assert.equal(
    getServiceTierBillingMultiplier({
      service_tier: 'ultrafast',
      service_tier_multiplier: 1,
    }),
    1
  )
  assert.equal(getServiceTierBillingMultiplier({ service_tier: 'fast' }), 2)
  assert.equal(getServiceTierLabel({ service_tier: 'priority' }), 'fast')
})

test('model prices use the recorded subscription and service tier multipliers once', () => {
  assert.equal(getModelPriceMultiplier(null), 1)
  assert.equal(getModelPriceMultiplier({ subscription_model_ratio: 1.2 }), 1.2)
  assert.equal(
    getModelPriceMultiplier({
      subscription_model_ratio: 1.2,
      service_tier_multiplier: 2.5,
    }),
    3
  )
  assert.equal(getModelPriceMultiplier({ subscription_model_ratio: 0 }), 0)
})
