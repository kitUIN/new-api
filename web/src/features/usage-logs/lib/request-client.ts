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
export interface RequestClientDisplay {
  id: string
  name: string
  icon: string
  url: string
}

export interface RequestClientConfig extends RequestClientDisplay {
  enabled: boolean
  rules: { header: string; pattern: string }[]
}

export function safeClientLink(value: string): string | undefined {
  try {
    const url = new URL(value)
    if (
      ['http:', 'https:'].includes(url.protocol) &&
      !url.username &&
      !url.password
    )
      return value
  } catch {
    /* Invalid links are not interactive. */
  }
  return undefined
}

export function safeClientImage(value: string): string | undefined {
  if (
    value.startsWith('/') &&
    !value.startsWith('//') &&
    !/[\\\r\n\t]/.test(value)
  )
    return value
  return safeClientLink(value)
}
