import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import {
  copyModelPricing,
  copyModelPricingSchema,
  type PricingMaps,
} from './copy-model-pricing'

type CopyModelPricingDialogProps = {
  sourceName: string
  pricingMaps: PricingMaps
  onChange: (field: string, value: string) => void
  onClose: () => void
}

export function CopyModelPricingDialog(props: CopyModelPricingDialogProps) {
  const { t } = useTranslation()
  const form = useForm({
    resolver: zodResolver(copyModelPricingSchema(props.pricingMaps)),
    defaultValues: { name: '' },
  })

  const onSubmit = form.handleSubmit(({ name }) => {
    const updates = copyModelPricing(props.pricingMaps, props.sourceName, name)
    if (Object.keys(updates).length === 0) {
      toast.error(t('Model not found'))
      return
    }
    for (const [key, value] of Object.entries(updates)) {
      props.onChange(key, value)
    }
    props.onClose()
    toast.success(
      t('Pricing changes saved to draft. Click "Save model prices" to apply.')
    )
  })

  return (
    <Dialog open onOpenChange={(open) => !open && props.onClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle className='pe-7 break-all'>
            {t('Copy {{name}} pricing', { name: props.sourceName })}
          </DialogTitle>
        </DialogHeader>
        <Form {...form}>
          <form onSubmit={onSubmit} className='grid gap-4'>
            <FormField
              control={form.control}
              name='name'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('New model name')}</FormLabel>
                  <FormControl>
                    <Input
                      {...field}
                      placeholder={`${props.sourceName}-copy`}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <DialogFooter>
              <DialogClose render={<Button type='button' variant='outline' />}>
                {t('Cancel')}
              </DialogClose>
              <Button type='submit'>{t('Copy')}</Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  )
}
