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
import { useId, useRef } from 'react'
import { useForm, useWatch } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import {
  Cancel01Icon,
  ImageAdd01Icon,
  SentIcon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import {
  Field,
  FieldError,
  FieldGroup,
  FieldLabel,
} from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import {
  MAX_TICKET_IMAGES,
  TICKET_IMAGE_TYPES,
  ticketSchema,
} from '../lib/schema'
import type { TicketInput } from '../types'
import { BlobImage } from './blob-image'

function ImagePreview(props: {
  file: File
  disabled: boolean
  onRemove: () => void
}) {
  const { t } = useTranslation()
  return (
    <div className='relative size-20 shrink-0'>
      <BlobImage
        blob={props.file}
        alt={props.file.name}
        className='bg-muted size-full rounded-md border object-contain'
      />
      <Button
        type='button'
        size='icon-xs'
        variant='secondary'
        className='absolute top-1 right-1'
        disabled={props.disabled}
        onClick={props.onRemove}
        aria-label={t('tickets.removeImage')}
        title={t('tickets.removeImage')}
      >
        <HugeiconsIcon icon={Cancel01Icon} />
      </Button>
    </div>
  )
}

export function MessageComposer(props: {
  creating?: boolean
  disabled?: boolean
  onSubmit: (input: TicketInput) => Promise<void>
}) {
  const { t } = useTranslation()
  const id = useId()
  const fileInput = useRef<HTMLInputElement>(null)
  const form = useForm<TicketInput>({
    resolver: zodResolver(ticketSchema(Boolean(props.creating))),
    defaultValues: { title: '', content: '', images: [] },
  })
  const images = useWatch({ control: form.control, name: 'images' })
  const disabled = Boolean(props.disabled || form.formState.isSubmitting)
  const submit = form.handleSubmit(async (values) => {
    try {
      await props.onSubmit(values)
      form.reset()
    } catch {
      // The shared API and mutation handlers display the server error; keep the draft.
    }
  })

  return (
    <form onSubmit={submit} className='flex min-w-0 flex-col gap-3'>
      <FieldGroup>
        {props.creating && (
          <Field data-invalid={Boolean(form.formState.errors.title)}>
            <FieldLabel htmlFor={`${id}-title`}>
              {t('tickets.subject')}
            </FieldLabel>
            <Input
              id={`${id}-title`}
              autoFocus
              disabled={disabled}
              aria-invalid={Boolean(form.formState.errors.title)}
              {...form.register('title')}
            />
            {form.formState.errors.title && (
              <FieldError>{t('tickets.titleInvalid')}</FieldError>
            )}
          </Field>
        )}
        <Field data-invalid={Boolean(form.formState.errors.content)}>
          <FieldLabel htmlFor={`${id}-content`}>
            {t('tickets.message')}
          </FieldLabel>
          <Textarea
            id={`${id}-content`}
            className='min-h-28 resize-y'
            disabled={disabled}
            aria-invalid={Boolean(form.formState.errors.content)}
            {...form.register('content')}
          />
          {form.formState.errors.content && (
            <FieldError>
              {t(
                form.formState.errors.content.message ??
                  'tickets.messageRequired'
              )}
            </FieldError>
          )}
        </Field>
        <Field data-invalid={Boolean(form.formState.errors.images)}>
          {images.length > 0 && (
            <div className='flex flex-wrap gap-2'>
              {images.map((file, index) => (
                <ImagePreview
                  key={`${file.name}-${file.lastModified}-${index}`}
                  file={file}
                  disabled={disabled}
                  onRemove={() =>
                    form.setValue(
                      'images',
                      images.filter((_, i) => i !== index),
                      { shouldValidate: true }
                    )
                  }
                />
              ))}
            </div>
          )}
          {form.formState.errors.images && (
            <FieldError>
              {t(
                form.formState.errors.images.message ?? 'tickets.imageInvalid'
              )}
            </FieldError>
          )}
        </Field>
      </FieldGroup>
      <div className='flex items-center justify-between gap-3'>
        <Tooltip>
          <TooltipTrigger
            render={
              <Button
                type='button'
                variant='outline'
                size='icon'
                disabled={disabled || images.length >= MAX_TICKET_IMAGES}
                aria-label={t('tickets.addImage')}
                onClick={() => fileInput.current?.click()}
              />
            }
          >
            <HugeiconsIcon icon={ImageAdd01Icon} />
          </TooltipTrigger>
          <TooltipContent>{t('tickets.addImage')}</TooltipContent>
        </Tooltip>
        <input
          ref={fileInput}
          type='file'
          multiple
          accept={TICKET_IMAGE_TYPES.join(',')}
          className='hidden'
          aria-label={t('tickets.addImage')}
          disabled={disabled}
          onChange={(event) => {
            const files = Array.from(event.target.files ?? [])
            if (images.length + files.length > MAX_TICKET_IMAGES) {
              form.setError('images', { message: 'tickets.imagesLimit' })
            } else {
              form.setValue('images', [...images, ...files], {
                shouldValidate: true,
              })
            }
            event.target.value = ''
          }}
        />
        <Button type='submit' disabled={disabled}>
          <HugeiconsIcon icon={SentIcon} data-icon='inline-start' />
          {t(props.creating ? 'tickets.create' : 'tickets.send')}
        </Button>
      </div>
    </form>
  )
}
