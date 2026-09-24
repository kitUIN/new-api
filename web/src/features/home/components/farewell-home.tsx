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
import { Link } from '@tanstack/react-router'
import { Gauge, Network, Send, ShieldCheck, Sparkles } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Separator } from '@/components/ui/separator'

const memoryItems = [
  {
    icon: Network,
    titleKey: 'Balance refunds, never disappear with funds',
    descriptionKey:
      'Recharge is closed from today, remaining funds will be fully refunded, and the operator will stand the final watch even at personal cost.',
    className: 'text-primary',
  },
  {
    icon: Gauge,
    titleKey: 'Open-source self-rescue, leave a spark',
    descriptionKey:
      'The site architecture and anti-blocking techniques will be fully open sourced, becoming sparks for peers still holding on.',
    className: 'text-info',
  },
  {
    icon: ShieldCheck,
    titleKey: 'Destroy everything, protect privacy',
    descriptionKey:
      'After shutdown, all logs and key records will be destroyed without traces to protect everyone’s final safety.',
    className: 'text-secondary-foreground',
  },
]

const themeGradientText =
  'bg-[linear-gradient(90deg,var(--primary),var(--info),var(--secondary))] bg-clip-text text-transparent'

const farewellSentenceKeys = [
  'Official access is expensive, and relay stations are besieged on all sides.',
  'We once tried, under official encirclement, to carve out a narrow gap for ordinary developers who could not afford AI.',
  'But facing relentless blocks and crushing costs, this shelter was finally forced into a dead end.',
  'Today, we have no choice but to bow to reality and say goodbye with dignity.',
]

const quoteSentenceKeys = [
  'Official is too expensive, relay is too fragile.',
  'We gave everything just so people without money could finish their work and keep going.',
  'Sadly, dawn has come, and beneath the high wall there is no place left for us.',
]

function DreamDoor() {
  return (
    <div
      aria-hidden='true'
      className='relative mx-auto h-44 w-80 max-w-full md:h-56 md:w-[28rem]'
    >
      <div className='bg-primary/15 absolute top-24 left-1/2 h-14 w-56 -translate-x-1/2 rounded-full blur-2xl md:top-32' />
      <div className='from-background via-secondary to-muted absolute top-8 left-1/2 h-32 w-24 -translate-x-1/2 rounded-t-full bg-linear-to-br shadow-[0_30px_60px_color-mix(in_oklch,var(--primary)_18%,transparent)] md:top-10 md:h-40 md:w-28'>
        <div className='from-info via-primary/45 to-secondary absolute inset-[10px] overflow-hidden rounded-t-full bg-linear-to-b'>
          <div className='bg-background/70 absolute top-10 left-3 h-8 w-20 rounded-full blur-sm' />
          <div className='bg-background/60 absolute top-16 -right-3 h-10 w-20 rounded-full blur-sm' />
          <div className='bg-background/20 absolute top-5 left-4 h-10 w-10 rounded-full' />
        </div>
      </div>
      <div className='from-card via-background to-muted absolute top-[3.35rem] left-[calc(50%-36px)] h-32 w-16 origin-left -skew-y-3 rounded-tl-full rounded-tr-2xl rounded-br-sm rounded-bl-sm bg-linear-to-br shadow-[14px_20px_30px_color-mix(in_oklch,var(--foreground)_12%,transparent)] md:top-[4rem] md:left-[calc(50%-42px)] md:h-40 md:w-20'>
        <div className='border-border bg-background/35 absolute top-8 left-4 h-10 w-7 rounded-t-full border md:top-10 md:left-5 md:h-12 md:w-8' />
        <div className='bg-border absolute top-20 left-4 h-px w-10 md:top-24 md:w-12' />
        <div className='border-border bg-background/25 absolute top-[5.5rem] left-4 h-9 w-10 rounded-sm border md:top-[6.7rem] md:h-10 md:w-12' />
        <div className='bg-warning absolute top-20 right-3 size-2 rounded-full shadow-[0_0_8px_color-mix(in_oklch,var(--warning)_70%,transparent)] md:top-24' />
      </div>
      <svg
        className='text-info/70 absolute top-6 left-[58%] h-20 w-36 overflow-visible md:top-7 md:h-24 md:w-44'
        viewBox='0 0 180 100'
        fill='none'
      >
        <path
          d='M12 70C36 44 66 92 82 56C92 34 58 30 62 54C66 84 117 34 150 16'
          stroke='currentColor'
          strokeWidth='2'
          strokeDasharray='7 8'
          strokeLinecap='round'
        />
        <path
          d='M146 12L176 2L160 36L154 21L146 12Z'
          fill='var(--card)'
          stroke='color-mix(in oklch, var(--info) 55%, transparent)'
        />
        <path
          d='M154 21L176 2'
          stroke='color-mix(in oklch, var(--info) 70%, transparent)'
        />
      </svg>
      <Sparkles className='text-background absolute top-10 left-[31%] size-4 drop-shadow-[0_0_6px_color-mix(in_oklch,var(--info)_75%,transparent)]' />
      <Sparkles className='text-background absolute top-24 right-[30%] size-3 drop-shadow-[0_0_6px_color-mix(in_oklch,var(--primary)_65%,transparent)]' />
      <span className='bg-background absolute top-4 left-[40%] size-1.5 rounded-full shadow-[0_0_8px_color-mix(in_oklch,var(--info)_70%,transparent)]' />
      <span className='bg-background absolute top-12 right-[38%] size-1 rounded-full shadow-[0_0_8px_color-mix(in_oklch,var(--primary)_70%,transparent)]' />
    </div>
  )
}

export function FarewellHome() {
  const { t } = useTranslation()

  return (
    <main className='bg-background text-foreground relative flex min-h-svh overflow-hidden'>
      <div
        aria-hidden='true'
        className='absolute inset-0 bg-[radial-gradient(circle_at_18%_8%,color-mix(in_oklch,var(--info)_22%,transparent),transparent_28%),radial-gradient(circle_at_80%_6%,color-mix(in_oklch,var(--primary)_14%,transparent),transparent_30%),linear-gradient(180deg,color-mix(in_oklch,var(--info)_8%,var(--background))_0%,var(--background)_52%,var(--background)_100%)]'
      />
      <div
        aria-hidden='true'
        className='absolute inset-x-0 bottom-0 h-64 bg-[radial-gradient(ellipse_at_center,color-mix(in_oklch,var(--primary)_14%,transparent),transparent_68%)]'
      />

      <div className='relative z-10 flex min-h-svh w-full flex-col px-5 py-7 sm:px-8'>
        <div className='border-border bg-card/70 text-primary landing-animate-fade-up inline-flex w-fit items-center gap-2 rounded-full px-4 py-2 text-sm font-semibold shadow-[0_8px_26px_color-mix(in_oklch,var(--primary)_14%,transparent)] backdrop-blur-md'>
          <span className='relative flex size-2'>
            <span className='bg-primary absolute inline-flex size-full animate-ping rounded-full opacity-60' />
            <span className='bg-primary relative inline-flex size-2 rounded-full' />
          </span>
          {t('AI Application Infrastructure Foundation')}
        </div>

        <section className='mx-auto flex w-full max-w-5xl flex-1 flex-col items-center justify-center pt-8 pb-8 text-center sm:pt-4'>
          <div
            className='landing-animate-scale-in'
            style={{ animationDelay: '60ms' }}
          >
            <DreamDoor />
          </div>

          <h1
            className='landing-animate-fade-up mt-1 font-serif text-[clamp(3rem,8vw,5.2rem)] leading-tight font-semibold tracking-[0.08em] opacity-0'
            style={{ animationDelay: '120ms' }}
          >
            <span>{t('We ran away prefix')}</span>
            <span className={themeGradientText}>{t('ran away highlight')}</span>
            <span>{t('We ran away suffix')}</span>
          </h1>
          <p
            className='landing-animate-fade-up mt-3 flex items-center justify-center gap-4 font-serif text-[clamp(1.35rem,3vw,2rem)] font-semibold tracking-[0.12em] opacity-0'
            style={{ animationDelay: '180ms' }}
          >
            <span className='from-info to-primary h-px w-4 bg-linear-to-r sm:w-6' />
            <span className={themeGradientText}>
              {t('May we meet in a better parallel world')}
            </span>
            <span className='from-primary to-secondary h-px w-4 bg-linear-to-r sm:w-6' />
          </p>

          <Card
            className='border-border bg-card/70 landing-animate-fade-up ring-border/70 mt-10 w-full max-w-3xl gap-0 rounded-lg py-0 text-left opacity-0 shadow-[0_22px_70px_color-mix(in_oklch,var(--foreground)_10%,transparent)] ring-1 backdrop-blur-xl'
            style={{ animationDelay: '240ms' }}
          >
            <CardHeader className='px-6 pt-6 pb-4 sm:px-8'>
              <CardTitle className='text-card-foreground flex items-center gap-3 text-xl font-medium'>
                <span className='bg-primary/10 flex size-7 items-center justify-center rounded-full text-lg shadow-inner'>
                  👋
                </span>
                {t('We tried our best, but the official version won')}
              </CardTitle>
              <CardDescription className='text-muted-foreground mt-4 flex flex-col text-[15px] leading-8'>
                {farewellSentenceKeys.map((key) => (
                  <span key={key}>{t(key)}</span>
                ))}
              </CardDescription>
            </CardHeader>
            <CardContent className='px-6 pb-6 sm:px-8'>
              <Separator className='bg-border/80 mb-6' />
              <div className='grid gap-5 md:grid-cols-3'>
                {memoryItems.map((item) => {
                  const Icon = item.icon
                  return (
                    <div
                      key={item.titleKey}
                      className='flex items-center gap-4'
                    >
                      <div className='border-border bg-muted/70 flex size-12 shrink-0 items-center justify-center rounded-full border shadow-sm'>
                        <span
                          className={`flex size-5 items-center justify-center ${item.className}`}
                        >
                          <Icon className='size-5 fill-current stroke-current' />
                        </span>
                      </div>
                      <div className='min-w-0'>
                        <h2 className='text-card-foreground text-sm font-semibold'>
                          {t(item.titleKey)}
                        </h2>
                        <p className='text-muted-foreground mt-1 text-sm leading-6'>
                          {t(item.descriptionKey)}
                        </p>
                      </div>
                    </div>
                  )
                })}
              </div>
            </CardContent>
          </Card>

          <blockquote
            className='text-muted-foreground landing-animate-fade-up mt-7 flex max-w-2xl items-center justify-center gap-5 font-serif text-base leading-8 opacity-0 sm:text-lg'
            style={{ animationDelay: '300ms' }}
          >
            <span className='text-primary/45 text-4xl leading-none'>“</span>
            <span className='flex flex-col'>
              {quoteSentenceKeys.map((key) => (
                <span key={key}>{t(key)}</span>
              ))}
            </span>
            <span className='text-primary/45 text-4xl leading-none'>”</span>
          </blockquote>

          <div
            className='landing-animate-fade-up mt-7 flex flex-wrap items-center justify-center gap-4 opacity-0'
            style={{ animationDelay: '360ms' }}
          >
            <Button
              className='bg-primary text-primary-foreground hover:bg-primary/85 h-12 min-w-40 rounded-full px-7 text-base shadow-[0_14px_28px_color-mix(in_oklch,var(--primary)_24%,transparent)] has-data-[icon=inline-start]:pl-7'
              render={
                <Link
                  to='/dashboard/$section'
                  params={{ section: 'overview' }}
                />
              }
            >
              <span className='flex w-full items-center justify-center gap-1.5'>
                <Send data-icon='inline-start' />
                <span>{t('Farewell with grace')}</span>
              </span>
            </Button>
          </div>
        </section>

        <footer className='text-muted-foreground/70 relative z-10 pb-1 text-center text-sm'>
          <span>
            © 2026 {t('Thank you for meeting us')} · {t('Move forward slowly')}{' '}
            · {t('See you again someday')}
          </span>
          <span className='text-muted-foreground/35 mx-2'>·</span>
          <a
            href='https://github.com/QuantumNous/new-api'
            target='_blank'
            rel='noopener noreferrer'
            className='text-foreground/70 hover:text-foreground font-medium transition-colors'
          >
            {t('New API')}
          </a>
        </footer>
      </div>
    </main>
  )
}
