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
import { Loader2, Plus } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { createInvitation } from '../api'
import {
  invitationFormSchema,
  type InvitationFormValues,
} from '../lib/invitation-form'

export function InvitationCreateCard() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const form = useForm<InvitationFormValues>({
    resolver: zodResolver(invitationFormSchema),
    defaultValues: { code: '', remark: '' },
  })

  const createMutation = useMutation({
    mutationFn: createInvitation,
    onSuccess: async (result) => {
      if (!result.success) return
      toast.success(t('Invitation code created successfully'))
      form.reset()
      await queryClient.invalidateQueries({ queryKey: ['invitations'] })
    },
  })

  const onSubmit = (values: InvitationFormValues) => {
    createMutation.mutate({
      code: values.code.trim(),
      remark: values.remark.trim(),
    })
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('Create Invitation Code')}</CardTitle>
        <CardDescription>
          {t(
            "Enter a QQ number manually. It will be used as the invitation code and the new user's username."
          )}
        </CardDescription>
      </CardHeader>
      <CardContent>
        <Form {...form}>
          <form
            onSubmit={form.handleSubmit(onSubmit)}
            className='grid gap-4 lg:grid-cols-[minmax(0,1fr)_minmax(0,2fr)_auto] lg:items-start'
          >
            <FormField
              control={form.control}
              name='code'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Invitation code (QQ number)')}</FormLabel>
                  <FormControl>
                    <Input
                      placeholder={t('Enter invitation code')}
                      inputMode='numeric'
                      autoComplete='off'
                      {...field}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='remark'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Remark')}</FormLabel>
                  <FormControl>
                    <Textarea
                      placeholder={t('Enter invitation remark')}
                      className='min-h-9 resize-y'
                      {...field}
                    />
                  </FormControl>
                  <p className='text-muted-foreground text-xs'>
                    {t(
                      "The remark will be copied to the user's remark after registration."
                    )}
                  </p>
                  <FormMessage />
                </FormItem>
              )}
            />

            <Button
              type='submit'
              className='gap-2 lg:mt-6'
              disabled={createMutation.isPending}
            >
              {createMutation.isPending ? (
                <Loader2 className='h-4 w-4 animate-spin' />
              ) : (
                <Plus className='h-4 w-4' />
              )}
              {t('Create invitation')}
            </Button>
          </form>
        </Form>
      </CardContent>
    </Card>
  )
}
