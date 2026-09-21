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
import { z } from 'zod'
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
import type { CostAction, CostInput } from '../types'

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
  const schema = z
    .object({
      month: z.string().regex(/^(?:20\d{2}|[3-9]\d{3})-(0[1-9]|1[0-2])$/),
      name: z.string().trim().max(128),
      amount: z.string(),
      remark: z.string().trim().max(1000),
      recurring: z.boolean(),
      scope: z.enum(['month', 'future']),
    })
    .superRefine((value, ctx) => {
      if (value.month > '9998-12') {
        ctx.addIssue({
          code: 'custom',
          path: ['month'],
          message: t('billingAudit.invalidMonth'),
        })
      }
      if (
        !removing &&
        (!value.name ||
          !/^\d+(\.\d{1,2})?$/.test(value.amount) ||
          Number(value.amount) <= 0 ||
          Number(value.amount) > 1_000_000_000)
      ) {
        ctx.addIssue({
          code: 'custom',
          path: ['amount'],
          message: t('billingAudit.invalidCost'),
        })
      }
      if (props.action.cost && value.month < props.action.cost.start_month) {
        ctx.addIssue({
          code: 'custom',
          path: ['month'],
          message: t('billingAudit.invalidMonth'),
        })
      }
    })
  const form = useForm<CostInput>({
    resolver: zodResolver(schema),
    defaultValues: {
      month: props.month,
      name: props.action.cost?.name ?? '',
      amount: props.action.cost
        ? (props.action.cost.amount_cents / 100).toFixed(2)
        : '',
      remark: props.action.cost?.remark ?? '',
      recurring: props.action.cost?.recurring ?? false,
      scope: 'month',
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
  const errors = Object.keys(form.formState.errors).length > 0

  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open && !mutation.isPending) props.onClose()
      }}
    >
      <DialogContent className='sm:max-w-lg'>
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription>
            {t('billingAudit.costDescription')}
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
                  onChange: () => form.setValue('month', props.month),
                })}
              >
                <option value='month'>{t('billingAudit.onlyMonth')}</option>
                <option value='future'>{t('billingAudit.fromMonth')}</option>
              </select>
            </div>
          )}
          <div className='space-y-2'>
            <Label htmlFor='cost-month'>
              {future
                ? t('billingAudit.effectiveMonth')
                : t('billingAudit.month')}
            </Label>
            <Input
              id='cost-month'
              type='month'
              min={props.action.cost?.start_month ?? '2000-01'}
              max='9998-12'
              readOnly={!creating && !future}
              {...form.register('month')}
            />
          </div>
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
                  {t('billingAudit.amountUSD')}
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
              {creating && (
                <label className='flex items-center gap-2 text-sm'>
                  <input type='checkbox' {...form.register('recurring')} />
                  {t('billingAudit.recurring')}
                </label>
              )}
            </>
          )}
          {future && (
            <p className='text-muted-foreground text-sm'>
              {t('billingAudit.futureHint')}
            </p>
          )}
          {removing && (
            <p className='text-destructive text-sm'>
              {future
                ? t('billingAudit.confirmStop')
                : t('billingAudit.confirmDelete')}
            </p>
          )}
          {errors && (
            <p role='alert' className='text-destructive text-sm'>
              {t('billingAudit.invalidCost')}
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
