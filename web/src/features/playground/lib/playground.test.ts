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
  DEFAULT_CONFIG,
  DEFAULT_PARAMETER_ENABLED,
  STORAGE_KEYS,
} from '../constants'
import { parameterSchema } from './parameters'
import { buildChatCompletionPayload } from './payload-builder'
import { resolvePlaygroundGroup, resolvePlaygroundModel } from './selection'
import { loadParameterEnabled, saveParameterEnabled } from './storage'

test('default requests omit temperature for both response modes', () => {
  for (const stream of [true, false]) {
    const payload = buildChatCompletionPayload(
      [],
      { ...DEFAULT_CONFIG, stream },
      DEFAULT_PARAMETER_ENABLED
    )
    assert.equal(Object.hasOwn(payload, 'temperature'), false)
    assert.equal(payload.stream, stream)
  }
})

test('opt-in parameters preserve explicit zero values and omit disabled values', () => {
  const values = { ...DEFAULT_CONFIG, temperature: 0, top_p: 0, seed: 0 }
  const enabled = {
    ...DEFAULT_PARAMETER_ENABLED,
    temperature: true,
    seed: true,
    top_p: false,
  }
  const payload = buildChatCompletionPayload([], values, enabled)
  assert.equal(payload.temperature, 0)
  assert.equal(payload.seed, 0)
  assert.equal(Object.hasOwn(payload, 'top_p'), false)
})

test('legacy temperature defaults migrate off while explicit new preferences persist', () => {
  const previous = Object.getOwnPropertyDescriptor(globalThis, 'localStorage')
  const saved = new Map<string, string>()
  Object.defineProperty(globalThis, 'localStorage', {
    configurable: true,
    value: {
      getItem: (key: string) => saved.get(key) ?? null,
      setItem: (key: string, value: string) => saved.set(key, value),
    },
  })
  try {
    assert.equal(loadParameterEnabled().temperature, undefined)
    saved.set(
      STORAGE_KEYS.PARAMETER_ENABLED,
      JSON.stringify({ temperature: true, max_tokens: true })
    )
    assert.deepEqual(loadParameterEnabled(), {
      temperature: false,
      max_tokens: true,
    })
    saveParameterEnabled({ temperature: true, seed: true })
    assert.deepEqual(loadParameterEnabled(), { temperature: true, seed: true })
    saveParameterEnabled(DEFAULT_PARAMETER_ENABLED)
    assert.equal(loadParameterEnabled().temperature, false)
  } finally {
    if (previous) Object.defineProperty(globalThis, 'localStorage', previous)
    else Reflect.deleteProperty(globalThis, 'localStorage')
  }
})

test('group changes cannot retain a model outside the current options', () => {
  const models = [
    { label: 'Model B', value: 'b' },
    { label: 'Shared', value: 'shared' },
  ]
  assert.equal(resolvePlaygroundModel(models, 'a'), 'b')
  assert.equal(resolvePlaygroundModel(models, 'shared'), 'shared')
  assert.equal(resolvePlaygroundModel([], 'a'), '')
  const groups = [
    { label: 'Free', value: 'free', ratio: 0 },
    { label: 'Default', value: 'default', ratio: 1 },
  ]
  assert.equal(resolvePlaygroundGroup(groups, 'free'), 'free')
  assert.equal(resolvePlaygroundGroup(groups, 'removed'), 'default')
  assert.equal(resolvePlaygroundGroup([], 'default'), '')
})

test('parameter validation accepts zero seed and rejects invalid ranges and fractional integers', () => {
  const defaults = {
    values: { ...DEFAULT_CONFIG, seed: 0 },
    enabled: DEFAULT_PARAMETER_ENABLED,
  }
  assert.equal(parameterSchema.safeParse(defaults).success, true)
  for (const invalid of [
    { temperature: 3 },
    { top_p: -1 },
    { max_tokens: 0 },
    { seed: 1.5 },
    { seed: NaN },
  ]) {
    assert.equal(
      parameterSchema.safeParse({
        ...defaults,
        values: { ...defaults.values, ...invalid },
      }).success,
      false
    )
  }
})
