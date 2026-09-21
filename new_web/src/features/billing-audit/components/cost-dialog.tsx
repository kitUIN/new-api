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
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { saveBillingCost } from '../api'
import { costSchema } from '../lib/cost-schema'
import type { CostAction, CostInput } from '../types'
import { CostPeriodFields } from './cost-period-fields'

interface Props {
  action: CostAction
  month: string
  onClose: () => void
}

export function CostDialog(props: Props) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const removing = props.action.kind === 'delete'
  const creating = props.action.kind === 'create'
  const cycleMonth = props.action.cost?.cycle_month || props.month
  const schema = costSchema(props.action, {
    invalidCost: t('billingAudit.invalidCost'),
    invalidMonth: t('billingAudit.invalidMonth'),
    invalidDates: t('billingAudit.invalidDates'),
  })
  const form = useForm<CostInput>({
    resolver: zodResolver(schema),
    defaultValues: {
      month: cycleMonth,
      name: props.action.cost?.name ?? '',
      amount: props.action.cost
        ? (
            (props.action.cost.period_amount_cents ??
              props.action.cost.amount_cents) / 100
          ).toFixed(2)
        : '',
      remark: props.action.cost?.remark ?? '',
      recurring: props.action.cost?.recurring ?? false,
      scope: 'month',
      allocation: props.action.cost?.allocation ?? 'month',
      start_date: props.action.cost?.period_start ?? '',
      end_date: props.action.cost?.period_end ?? '',
    },
  })
  const mutation = useMutation({
    mutationFn: (input: CostInput) => saveBillingCost(props.action, input),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['billing-audit'] })
      toast.success(t('billingAudit.saved'))
      props.onClose()
    },
  })
  let title = t('billingAudit.editCost')
  if (creating) title = t('billingAudit.addCost')
  if (removing) title = t('billingAudit.removeCost')
  const scope = form.watch('scope')
  const future = !creating && scope === 'future'
  const subscription = form.watch('allocation') === 'subscription'
  let scopeLabel = t('billingAudit.onlyMonth')
  let futureLabel = t('billingAudit.fromMonth')
  let monthLabel = future
    ? t('billingAudit.effectiveMonth')
    : t('billingAudit.month')
  let removeHint = future
    ? t('billingAudit.confirmStop')
    : t('billingAudit.confirmDelete')
  if (subscription) {
    scopeLabel = t('billingAudit.onlyCycle')
    futureLabel = t('billingAudit.fromCycle')
    monthLabel = t('billingAudit.cycleMonth')
    removeHint = future
      ? t('billingAudit.confirmStopSubscription')
      : t('billingAudit.confirmDeleteCycle')
  }
  const errors = Object.keys(form.formState.errors).length > 0

  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open && !mutation.isPending) props.onClose()
      }}
    >
      <DialogContent className='max-h-[90dvh] overflow-y-auto sm:max-w-lg'>
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription>
            {subscription
              ? t('billingAudit.subscriptionDescription')
              : t('billingAudit.costDescription')}
          </DialogDescription>
        </DialogHeader>
        <form
          className='space-y-4'
          onSubmit={form.handleSubmit((values) => mutation.mutate(values))}
        >
          {!creating && props.action.cost?.recurring && (
            <div className='space-y-2'>
              <Label htmlFor='cost-scope'>{t('billingAudit.scope')}</Label>
              <select
                id='cost-scope'
                className='border-input bg-background h-9 w-full rounded-md border px-3'
                {...form.register('scope', {
                  onChange: (event) =>
                    form.setValue(
                      'month',
                      event.target.value === 'future' ? props.month : cycleMonth
                    ),
                })}
              >
                <option value='month'>{scopeLabel}</option>
                <option value='future'>{futureLabel}</option>
              </select>
            </div>
          )}
          {creating && <CostPeriodFields form={form} month={props.month} />}
          {!creating && (
            <div className='space-y-2'>
              <Label htmlFor='cost-month'>{monthLabel}</Label>
              <Input
                id='cost-month'
                type='month'
                min={props.action.cost?.start_month ?? '2000-01'}
                max='9998-12'
                readOnly={!creating && !future}
                {...form.register('month')}
              />
            </div>
          )}
          {!creating && subscription && (
            <p className='text-muted-foreground text-sm'>
              {props.action.cost?.period_start} →{' '}
              {props.action.cost?.period_end} ·{' '}
              {t('billingAudit.editCycleHint')}
            </p>
          )}
          {!removing && (
            <>
              <div className='space-y-2'>
                <Label htmlFor='cost-name'>{t('billingAudit.costName')}</Label>
                <Input
                  id='cost-name'
                  maxLength={128}
                  {...form.register('name')}
                />
              </div>
              <div className='space-y-2'>
                <Label htmlFor='cost-amount'>
                  {subscription
                    ? t('billingAudit.periodAmount')
                    : t('billingAudit.amountUSD')}
                </Label>
                <Input
                  id='cost-amount'
                  inputMode='decimal'
                  placeholder='0.00'
                  {...form.register('amount')}
                />
              </div>
              <div className='space-y-2'>
                <Label htmlFor='cost-remark'>{t('billingAudit.remark')}</Label>
                <Input
                  id='cost-remark'
                  maxLength={1000}
                  {...form.register('remark')}
                />
              </div>
            </>
          )}
          {future && (
            <p className='text-muted-foreground text-sm'>
              {subscription
                ? t('billingAudit.futureCycleHint')
                : t('billingAudit.futureHint')}
            </p>
          )}
          {removing && <p className='text-destructive text-sm'>{removeHint}</p>}
          {errors && (
            <p role='alert' className='text-destructive text-sm'>
              {form.formState.errors.end_date?.message ||
                form.formState.errors.month?.message ||
                t('billingAudit.invalidCost')}
            </p>
          )}
          {mutation.isError && (
            <p role='alert' className='text-destructive text-sm'>
              {t('billingAudit.saveFailed')}
            </p>
          )}
          <DialogFooter>
            <Button
              type='button'
              variant='outline'
              disabled={mutation.isPending}
              onClick={props.onClose}
            >
              {t('Cancel')}
            </Button>
            <Button
              type='submit'
              variant={removing ? 'destructive' : 'default'}
              disabled={mutation.isPending}
            >
              {removing ? t('Confirm') : t('Save')}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
