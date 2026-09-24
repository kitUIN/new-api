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
import { useWatch } from 'react-hook-form'
import type { UseFormReturn } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { nextBillingDate } from '../lib'
import type { CostInput } from '../types'

export function CostPeriodFields(props: {
  form: UseFormReturn<CostInput>
  month: string
}) {
  const { t } = useTranslation()
  const allocation = useWatch({
    control: props.form.control,
    name: 'allocation',
  })
  const recurring = useWatch({ control: props.form.control, name: 'recurring' })
  const subscription = allocation === 'subscription'
  return (
    <>
      <div className='space-y-2'>
        <Label htmlFor='cost-allocation'>{t('billingAudit.allocation')}</Label>
        <select
          id='cost-allocation'
          className='border-input bg-background h-9 w-full rounded-md border px-3'
          {...props.form.register('allocation', {
            onChange: (event) => {
              const selectedMonth = props.form.getValues('month') || props.month
              const start =
                event.target.value === 'subscription'
                  ? `${selectedMonth}-01`
                  : ''
              props.form.setValue('start_date', start)
              props.form.setValue('end_date', nextBillingDate(start))
              props.form.setValue('month', selectedMonth)
            },
          })}
        >
          <option value='month'>{t('billingAudit.calendarAllocation')}</option>
          <option value='subscription'>
            {t('billingAudit.subscriptionAllocation')}
          </option>
        </select>
      </div>
      {subscription ? (
        <>
          <div className='grid grid-cols-2 gap-3'>
            <div className='space-y-2'>
              <Label htmlFor='cost-start'>
                {t('billingAudit.periodStart')}
              </Label>
              <Input
                id='cost-start'
                type='date'
                min='2000-01-01'
                max='9998-12-31'
                {...props.form.register('start_date', {
                  onChange: (event) => {
                    props.form.setValue(
                      'end_date',
                      nextBillingDate(event.target.value)
                    )
                    if (event.target.value)
                      props.form.setValue(
                        'month',
                        event.target.value.slice(0, 7)
                      )
                  },
                })}
              />
            </div>
            <div className='space-y-2'>
              <Label htmlFor='cost-end'>{t('billingAudit.periodEnd')}</Label>
              <Input
                id='cost-end'
                type='date'
                min='2000-01-01'
                max='9998-12-31'
                readOnly={recurring}
                {...props.form.register('end_date')}
              />
            </div>
          </div>
          <p className='text-muted-foreground text-sm'>
            {t('billingAudit.prorationHint')}
          </p>
        </>
      ) : (
        <div className='space-y-2'>
          <Label htmlFor='cost-month'>{t('billingAudit.month')}</Label>
          <Input
            id='cost-month'
            type='month'
            min='2000-01'
            max='9998-12'
            {...props.form.register('month')}
          />
        </div>
      )}
      <label className='flex items-center gap-2 text-sm'>
        <input
          type='checkbox'
          {...props.form.register('recurring', {
            onChange: (event) => {
              if (event.target.checked && subscription)
                props.form.setValue(
                  'end_date',
                  nextBillingDate(props.form.getValues('start_date'))
                )
            },
          })}
        />
        {t('billingAudit.recurring')}
      </label>
      {subscription && recurring && (
        <p className='text-muted-foreground text-sm'>
          {t('billingAudit.renewalHint')}
        </p>
      )}
    </>
  )
}
