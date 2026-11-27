import assert from 'node:assert/strict'
import { test } from 'node:test'
import { getLogTokensPerSecond } from './format'

test('subtracts time to first token using millisecond timing for short requests', () => {
  const log = { use_time: 0, completion_tokens: 30 }
  const other = { total_ms: 800, first_response_ms: 200 }
  assert.equal(getLogTokensPerSecond(log, other), 50)
})

test('uses legacy timing when available and ignores an unrecorded first response', () => {
  const log = { use_time: 10, completion_tokens: 120 }
  assert.equal(getLogTokensPerSecond(log, { frt: 4000 }), 20)
  assert.equal(getLogTokensPerSecond(log, { frt: -1000 }), 12)
  assert.equal(getLogTokensPerSecond(log, null), 12)
})

test('omits throughput without output tokens or a positive generation interval', () => {
  const log = { use_time: 1, completion_tokens: 10 }
  assert.equal(getLogTokensPerSecond(log, { frt: 1000 }), null)
  assert.equal(getLogTokensPerSecond(log, { frt: 1200 }), null)
  assert.equal(
    getLogTokensPerSecond({ ...log, completion_tokens: 0 }, null),
    null
  )
})
