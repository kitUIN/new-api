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
import { channelSchema } from '../types'
import {
  CHANNEL_FORM_DEFAULT_VALUES,
  channelFormSchema,
  transformChannelToFormDefaults,
  transformFormDataToCreatePayload,
  transformFormDataToUpdatePayload,
  transformProviderToCreateDefaults,
} from './channel-form'

test('Claude Code restriction survives create, edit, channel type change and disable', () => {
  for (const type of [1, 14, 33]) {
    const form = channelFormSchema.parse({
      ...CHANNEL_FORM_DEFAULT_VALUES,
      type,
      name: 'Client restriction',
      models: 'claude-test',
      key: 'test-key',
      other: 'test',
      remark: 'test',
      group: ['default'],
      disable_claude_code: true,
      settings: JSON.stringify({ custom_setting: 'preserved' }),
    })
    const payload = transformFormDataToCreatePayload(form).channel
    assert.equal(JSON.parse(payload.settings!).disable_claude_code, true)

    const stored = channelSchema.parse({
      ...payload,
      id: 1,
      created_time: 0,
      test_time: 0,
      response_time: 0,
      balance_updated_time: 0,
    })
    const edit = transformChannelToFormDefaults(stored)
    assert.equal(edit.disable_claude_code, true)
    const updated = transformFormDataToUpdatePayload(
      { ...edit, type: 14, disable_claude_code: false },
      stored.id
    )
    const settings = JSON.parse(updated.settings!)
    assert.notEqual(settings.disable_claude_code, true)
    assert.equal(settings.custom_setting, 'preserved')
    assert.equal(
      transformChannelToFormDefaults({ ...stored, settings: '{}' })
        .disable_claude_code,
      false
    )
  }
})

test('new channels omit group membership in every creation mode', () => {
  const sourceChannel = channelSchema.parse({
    id: 1,
    name: 'Existing channel',
    key: 'test-key',
    type: 14,
    status: 1,
    models: 'claude-sonnet-5-5',
    group:
      'codex-pro-正价菲区,codex-pro-正价luna,grok-正价,codex-pro正价,codex-pro-正价,codex-plus-正价luna',
    created_time: 0,
    test_time: 0,
    response_time: 0,
    balance_updated_time: 0,
  })
  const providerDefaults = transformProviderToCreateDefaults({
    id: 'provider-58',
    key: 'provider-58',
    is_provider: true,
    provider_id: 58,
    name: 'Provider',
    base_url: 'http://provider.example',
    status: 1,
    group: sourceChannel.group,
    used_quota: 0,
    response_time: 0,
    priority: null,
    weight: null,
    channel_count: 1,
    enabled_count: 1,
    children: [sourceChannel],
  })

  assert.equal(Object.hasOwn(CHANNEL_FORM_DEFAULT_VALUES, 'group'), false)
  assert.equal(Object.hasOwn(providerDefaults, 'group'), false)
  assert.equal(
    Object.hasOwn(transformChannelToFormDefaults(sourceChannel), 'group'),
    false
  )

  for (const mode of ['single', 'batch', 'multi_to_single'] as const) {
    const form = {
      ...providerDefaults,
      name: 'New channel',
      key: 'test-key',
      multi_key_mode: mode,
      // Simulate stale form state left by an older client.
      group: sourceChannel.group.split(','),
    }
    const payload = transformFormDataToCreatePayload(form)
    const request = JSON.parse(JSON.stringify(payload))
    assert.equal(request.mode, mode)
    assert.equal(Object.hasOwn(request.channel, 'group'), false)
    assert.equal(request.channel.provider_id, 58)
    assert.equal(request.channel.models, sourceChannel.models)
    assert.equal(request.channel.base_url, 'http://provider.example')
    assert.equal(Object.hasOwn(channelFormSchema.parse(form), 'group'), false)
    assert.equal(
      Object.hasOwn(transformFormDataToUpdatePayload(form, 1), 'group'),
      false
    )
  }
})
