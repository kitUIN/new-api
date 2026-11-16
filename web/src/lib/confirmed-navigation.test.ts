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
import { afterEach, beforeEach, test } from 'node:test'
import { useNavigationConfirmationStore } from '@/stores/navigation-confirmation-store'
import { submitPaymentForm } from '@/features/wallet/lib/payment'
import {
  handleNavigationClick,
  openWithConfirmation,
  requiresNavigationConfirmation,
} from './confirmed-navigation'

const currentUrl = 'https://gateway.example/keys'
const store = useNavigationConfirmationStore
const opened: unknown[][] = []
const assigned: string[] = []
const originals = new Map<string, PropertyDescriptor | undefined>()

class TestAnchor {
  target = ''
  download = false
  relList = { contains: (value: string) => value === 'noreferrer' }

  constructor(public href: string) {}

  hasAttribute(name: string): boolean {
    return name === 'href' || (name === 'download' && this.download)
  }
}

beforeEach(() => {
  opened.length = 0
  assigned.length = 0
  for (const [key, value] of Object.entries({
    window: {
      location: {
        href: currentUrl,
        assign: (url: string) => assigned.push(url),
      },
      open: (...args: unknown[]) => opened.push(args),
    },
    document: { baseURI: currentUrl, querySelector: () => null },
    navigator: { userAgent: 'Chrome' },
    HTMLAnchorElement: TestAnchor,
  })) {
    originals.set(key, Object.getOwnPropertyDescriptor(globalThis, key))
    Object.defineProperty(globalThis, key, { configurable: true, value })
  }
})

afterEach(() => {
  store.getState().cancel()
  for (const [key, descriptor] of originals) {
    if (descriptor) Object.defineProperty(globalThis, key, descriptor)
    else Reflect.deleteProperty(globalThis, key)
  }
})

function click(
  link: TestAnchor,
  options: Partial<MouseEvent> = {}
): MouseEvent {
  const event = new Event('click', { cancelable: true })
  return Object.assign(event, {
    button: 0,
    ctrlKey: false,
    metaKey: false,
    shiftKey: false,
    composedPath: () => [{}, link],
    ...options,
  }) as MouseEvent
}

test('ordinary internal routes and fragments stay in the current page', () => {
  for (const url of ['/pricing', '#details', currentUrl]) {
    assert.equal(
      requiresNavigationConfirmation(url, '_self', currentUrl),
      false
    )
  }
  const event = click(new TestAnchor('https://gateway.example/pricing'))
  handleNavigationClick(event)
  assert.equal(event.defaultPrevented, false)
  assert.equal(store.getState().pending, null)
})

test('external origins, protocols, named windows and internal new tabs require confirmation', () => {
  for (const [url, target] of [
    ['https://docs.example/', '_self'],
    ['//docs.example/', '_self'],
    ['http://gateway.example/', '_self'],
    ['https://gateway.example:8443/', '_self'],
    ['mailto:help@example.com', '_self'],
    ['ccswitch://import?name=test', '_blank'],
    ['/pricing', '_blank'],
    ['/pricing', 'preview'],
  ]) {
    assert.equal(requiresNavigationConfirmation(url, target, currentUrl), true)
  }
})

test('canceling a programmatic open never creates a tab', async () => {
  const result = openWithConfirmation('https://docs.example/', '_blank')
  assert.equal(opened.length, 0)
  store.getState().cancel()
  assert.equal(await result, false)
  assert.equal(opened.length, 0)
})

test('confirmation opens once synchronously with the original target and features', async () => {
  const result = openWithConfirmation(
    '/pricing',
    '_blank',
    'noopener,noreferrer'
  )
  assert.equal(store.getState().pending?.url, 'https://gateway.example/pricing')
  store.getState().confirm()
  assert.deepEqual(opened, [['/pricing', '_blank', 'noopener,noreferrer']])
  store.getState().confirm()
  assert.equal(await result, true)
  assert.equal(opened.length, 1)
})

test('OAuth binding retains opener behavior when no window features were supplied', async () => {
  const result = openWithConfirmation('https://oauth.example/', '_blank')
  store.getState().confirm()
  assert.equal(await result, true)
  assert.equal(opened[0][2], undefined)
})

test('same-tab navigation waits for confirmation and cancellation keeps the current page', async () => {
  const canceled = openWithConfirmation('https://checkout.example/', '_self')
  store.getState().cancel()
  assert.equal(await canceled, false)
  assert.deepEqual(assigned, [])
  const confirmed = openWithConfirmation('https://checkout.example/', '_self')
  store.getState().confirm()
  assert.deepEqual(assigned, ['https://checkout.example/'])
  assert.equal(await confirmed, true)
})

test('another request cannot replace the destination currently being reviewed', async () => {
  const first = openWithConfirmation('https://first.example/')
  const second = openWithConfirmation('https://second.example/')
  assert.equal(await second, false)
  assert.equal(store.getState().pending?.url, 'https://first.example/')
  store.getState().confirm()
  assert.equal(await first, true)
  assert.equal(opened[0][0], 'https://first.example/')
})

test('external anchor clicks are stopped until confirmation', () => {
  const event = click(new TestAnchor('https://docs.example/'))
  handleNavigationClick(event)
  assert.equal(event.defaultPrevented, true)
  assert.equal(assigned.length, 0)
  store.getState().confirm()
  assert.deepEqual(assigned, ['https://docs.example/'])
})

test('modified clicks and middle clicks confirm opening internal links in a new tab', () => {
  for (const options of [
    { ctrlKey: true },
    { metaKey: true },
    { shiftKey: true },
    { button: 1 },
  ]) {
    const event = click(
      new TestAnchor('https://gateway.example/pricing'),
      options
    )
    handleNavigationClick(event)
    assert.equal(event.defaultPrevented, true)
    assert.equal(store.getState().pending?.target, '_blank')
    store.getState().cancel()
  }
  assert.equal(opened.length, 0)
})

test('downloads, right clicks and already canceled events keep their existing behavior', () => {
  const download = new TestAnchor('https://files.example/export.csv')
  download.download = true
  const canceled = click(new TestAnchor('https://docs.example/'))
  canceled.preventDefault()
  for (const event of [
    click(download),
    click(new TestAnchor('https://docs.example/'), { button: 2 }),
    canceled,
  ]) {
    handleNavigationClick(event)
    assert.equal(store.getState().pending, null)
  }
})

test('executable URLs are never opened', async () => {
  assert.equal(await openWithConfirmation('javascript:alert(1)'), false)
  assert.equal(store.getState().pending, null)
  assert.equal(opened.length, 0)
})

test('payment POST forms are only created and submitted after confirmation', async () => {
  const inputs: { name: string; value: string }[] = []
  let created = 0
  let submitted = 0
  let attached = false
  const form = {
    action: '',
    method: '',
    target: '',
    appendChild: (input: { name: string; value: string }) => inputs.push(input),
    submit: () => {
      assert.equal(attached, true)
      submitted++
    },
  }
  Object.assign(document, {
    createElement: (tag: string) => {
      created++
      return tag === 'form' ? form : { type: '', name: '', value: '' }
    },
    body: {
      appendChild: () => {
        attached = true
      },
      removeChild: () => {
        attached = false
      },
    },
  })
  const canceled = submitPaymentForm('https://checkout.example/pay', {
    order: 42,
  })
  assert.equal(created, 0)
  store.getState().cancel()
  assert.equal(await canceled, false)
  assert.equal(created, 0)

  const confirmed = submitPaymentForm('https://checkout.example/pay', {
    order: 42,
  })
  store.getState().confirm()
  assert.equal(await confirmed, true)
  assert.equal(submitted, 1)
  assert.equal(form.action, 'https://checkout.example/pay')
  assert.equal(form.method, 'POST')
  assert.equal(form.target, '_blank')
  assert.equal(inputs[0].name, 'order')
  assert.equal(inputs[0].value, '42')
  assert.equal(attached, false)
})
