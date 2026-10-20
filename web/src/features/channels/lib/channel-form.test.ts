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
