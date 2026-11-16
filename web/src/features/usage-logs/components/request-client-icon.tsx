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
import { useState, type ReactNode } from 'react'
import {
  ClaudeCode,
  CodeBuddy,
  DeepSeek,
  OpenAI,
  OpenCode,
  Pi,
} from '@lobehub/icons'
import { Monitor, Terminal } from 'lucide-react'
import { VscVscode } from 'react-icons/vsc'
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import {
  safeClientImage,
  safeClientLink,
  type RequestClientDisplay,
} from '../lib/request-client'

function CodexVariantIcon(props: { badge: ReactNode }) {
  return (
    <span className='relative inline-flex size-4' aria-hidden='true'>
      <OpenAI size={16} />
      <span className='bg-background ring-background absolute -right-1 -bottom-0.5 inline-flex size-2.5 rounded-[2px] ring-1'>
        {props.badge}
      </span>
    </span>
  )
}

const brandIcons = new Map([
  ['codex', <OpenAI size={16} aria-hidden='true' />],
  [
    'codex-desktop',
    <CodexVariantIcon badge={<Monitor size={10} strokeWidth={2.5} />} />,
  ],
  [
    'codex-tui',
    <CodexVariantIcon badge={<Terminal size={10} strokeWidth={2.5} />} />,
  ],
  [
    'codex-vscode',
    <CodexVariantIcon
      badge={<VscVscode size={10} className='text-sky-500' />}
    />,
  ],
  [
    'codex-tui-vscode',
    <CodexVariantIcon
      badge={
        <Terminal
          size={10}
          strokeWidth={2.5}
          className='bg-sky-600 text-white'
        />
      }
    />,
  ],
  ['claude-code', <ClaudeCode.Color size={16} aria-hidden='true' />],
  ['pi', <Pi size={16} aria-hidden='true' />],
  ['opencode', <OpenCode size={16} aria-hidden='true' />],
  ['codebuddy', <CodeBuddy.Color size={16} aria-hidden='true' />],
  ['deepseek-harness', <DeepSeek size={16} aria-hidden='true' />],
])

export function RequestClientIcon(props: { client?: RequestClientDisplay }) {
  const [failedImage, setFailedImage] = useState('')
  if (!props.client?.name) return null
  const client = props.client
  const brandIcon = brandIcons.get(client.icon)
  const src = safeClientImage(client.icon)
  if (!brandIcon && (!src || failedImage === src)) return null
  const icon = brandIcon ? (
    <span
      className='pointer-events-none inline-flex shrink-0'
      role='img'
      aria-label={client.name}
    >
      {brandIcon}
    </span>
  ) : (
    <img
      src={src}
      alt={client.name}
      className='pointer-events-none size-4 shrink-0 rounded-sm bg-white object-contain'
      referrerPolicy='no-referrer'
      onError={() => setFailedImage(src ?? '')}
    />
  )
  const href = safeClientLink(client.url)
  return (
    <TooltipProvider delay={300}>
      <Tooltip>
        <TooltipTrigger
          render={
            href ? (
              <a href={href} target='_blank' rel='noopener noreferrer' />
            ) : (
              <span tabIndex={0} />
            )
          }
          aria-label={client.name}
          className='inline-flex shrink-0 rounded-sm outline-offset-2 hover:opacity-70 focus-visible:outline-2'
          onClick={(event) => event.stopPropagation()}
        >
          {icon}
        </TooltipTrigger>
        <TooltipContent>{client.name}</TooltipContent>
      </Tooltip>
    </TooltipProvider>
  )
}
