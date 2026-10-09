import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import messages from '@/i18n/locales/zh.json'
import { createInstance } from 'i18next'
import assert from 'node:assert/strict'
import { test } from 'node:test'
import { renderToStaticMarkup } from 'react-dom/server'
import { I18nextProvider } from 'react-i18next'
import { auditMoney } from '../lib'
import type { BillingDailyTopUp } from '../types'
import { DailyTopUpsTable } from './daily-topups-table'

async function renderDailyTable(
  items: BillingDailyTopUp[],
  total: number,
  month = '2026-10'
): Promise<string> {
  const client = new QueryClient({
    defaultOptions: {
      queries: { retry: false, staleTime: Infinity, gcTime: Infinity },
    },
  })
  client.setQueryData(['billing-audit', 'daily-topups', '2026-10', 1], {
    items,
    total,
  })
  const translations = createInstance()
  await translations.init({ lng: 'zh', resources: { zh: messages } })
  return renderToStaticMarkup(
    <I18nextProvider i18n={translations}>
      <QueryClientProvider client={client}>
        <DailyTopUpsTable month={month} />
      </QueryClientProvider>
    </I18nextProvider>
  )
}

const row: BillingDailyTopUp = {
  date: '2026-10-02',
  record_count: 42,
  received: '10',
  refunded: '16.000002',
  net_recharge: '-6.000002',
  unconverted_count: 1,
}

test('daily view renders receipts, refunds, negative net and legacy warnings', async () => {
  const markup = await renderDailyTable([row], 21)
  assert.ok(markup.includes(row.date))
  assert.ok(markup.includes('记录数'))
  assert.ok(markup.includes(auditMoney(row.received, 6)))
  assert.ok(markup.includes(auditMoney(row.refunded, 6)))
  assert.ok(markup.includes(auditMoney(row.net_recharge, 6)))
  assert.ok(markup.includes('有 1 笔历史金额无法换算'))
  assert.ok(markup.includes('1 / 2 · 共 21 天'))
})

test('daily view renders an empty state', async () => {
  const markup = await renderDailyTable([], 0)
  assert.ok(markup.includes(messages.translation['billingAudit.noTopups']))
  assert.ok(markup.includes('1 / 1 · 共 0 天'))
})

test('daily view does not show cached rows from another month', async () => {
  const markup = await renderDailyTable([row], 1, '2026-11')
  assert.ok(!markup.includes(row.date))
  assert.ok(markup.includes('role="status"'))
})
