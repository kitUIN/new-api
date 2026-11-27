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
import assert from 'node:assert/strict'
import { test } from 'node:test'
import {
  formatChannelAffinitySessionKey,
  getChannelAffinitySessionKey,
  getLogTypeConfig,
  getUsageLogSessionKeyInfo,
} from './utils'

test('usage log labels distinguish client, stream and unattributed interruptions', () => {
  assert.equal(
    getLogTypeConfig(2, 'user_interrupted').label,
    'usageLogs.userInterrupted'
  )
  assert.equal(
    getLogTypeConfig(5, 'stream_interrupted').label,
    'usageLogs.streamInterrupted'
  )
  assert.equal(
    getLogTypeConfig(5, 'interrupted').label,
    'usageLogs.interrupted'
  )
  assert.equal(getLogTypeConfig(5, 'failed').label, 'Error')
  assert.equal(getLogTypeConfig(2, 'completed').label, 'Consume')
  assert.equal(getLogTypeConfig(8, 'stream_interrupted').color, 'neutral')
  assert.equal(getLogTypeConfig(9, 'rate_limited').label, 'Rate Limited')
})

test('session keys retain the complete value for copying while display stays compact', () => {
  const fullKey = 'session-abcdefghijklmnopqrstuvwxyz'
  const key = getChannelAffinitySessionKey(
    getUsageLogSessionKeyInfo(
      { key_value: fullKey, key_hint: 'sess...wxyz', key_fp: '12345678' },
      { key_hint: 'old-hint', key_fp: '87654321' }
    )
  )
  assert.equal(key, fullKey)
  assert.equal(formatChannelAffinitySessionKey(key), 'sess...wxyz')
})

test('older logs retain their available session hint or fingerprint', () => {
  assert.equal(
    getChannelAffinitySessionKey(
      getUsageLogSessionKeyInfo(undefined, {
        key_hint: 'sess...wxyz',
        key_fp: '12345678',
      })
    ),
    'sess...wxyz'
  )
  assert.equal(
    getChannelAffinitySessionKey({ key_value: '', key_fp: '12345678' }),
    '12345678'
  )
  assert.equal(getChannelAffinitySessionKey(null), '')
})
