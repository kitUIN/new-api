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
import { renderToStaticMarkup } from 'react-dom/server'
import { safeClientImage, safeClientLink } from '../lib/request-client'
import { RequestClientIcon } from './request-client-icon'

test('unidentified clients have no placeholder icon', () => {
  assert.equal(renderToStaticMarkup(<RequestClientIcon />), '')
})

test('Codex variants have distinct visible marks and accessible names', () => {
  const rendered = [
    'codex-desktop',
    'codex-tui',
    'codex-vscode',
    'codex-tui-vscode',
  ].map((id) => {
    const html = renderToStaticMarkup(
      <RequestClientIcon client={{ id, name: id, icon: id, url: '' }} />
    )
    assert.match(html, new RegExp(`aria-label="${id}"`))
    assert.ok((html.match(/<svg/g) ?? []).length >= 2)
    return html
  })
  assert.equal(new Set(rendered).size, 4)
})

test('configured icons have an accessible name and a safe separate destination', () => {
  const html = renderToStaticMarkup(
    <RequestClientIcon
      client={{
        id: 'custom',
        name: 'Custom Client',
        icon: 'codex',
        url: 'https://example.com/client',
      }}
    />
  )
  assert.match(html, /href="https:\/\/example.com\/client"/)
  assert.match(html, /target="_blank"/)
  assert.match(html, /rel="noopener noreferrer"/)
  assert.match(html, /aria-label="Custom Client"/)
})

test('untrusted URLs cannot become executable links or images', () => {
  for (const url of [
    'javascript:alert(1)',
    'data:text/html,test',
    '//evil.example/icon',
    '/\\evil.example/icon',
  ]) {
    assert.equal(safeClientLink(url), undefined)
    assert.equal(safeClientImage(url), undefined)
  }
  assert.equal(
    safeClientImage('/images/clients/magpie.svg'),
    '/images/clients/magpie.svg'
  )
  const html = renderToStaticMarkup(
    <RequestClientIcon
      client={{
        id: 'custom',
        name: 'Custom',
        icon: 'codex',
        url: 'javascript:alert(1)',
      }}
    />
  )
  assert.doesNotMatch(html, /<a /)
  assert.match(html, /<svg/)
})
