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
import { Globe02Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import {
  AU,
  CA,
  CN,
  DE,
  FR,
  GB,
  HK,
  IN,
  JP,
  KR,
  NL,
  RU,
  SG,
  TW,
  US,
  VN,
} from 'country-flag-icons/react/3x2'

const regions = [
  { flag: HK, pattern: /香港|hong\s*kong|\bhk\b/i },
  { flag: TW, pattern: /台湾|台灣|taiwan|\btw\b/i },
  { flag: JP, pattern: /日本|东京|大阪|japan|tokyo|osaka|\bjp\b/i },
  { flag: SG, pattern: /新加坡|狮城|singapore|\bsg\b/i },
  {
    flag: US,
    pattern:
      /美国|美國|洛杉矶|圣何塞|united states|america|los angeles|\bus\b|\busa\b/i,
  },
  { flag: KR, pattern: /韩国|韓國|首尔|korea|seoul|\bkr\b/i },
  { flag: GB, pattern: /英国|英國|伦敦|united kingdom|london|\buk\b|\bgb\b/i },
  { flag: DE, pattern: /德国|德國|法兰克福|germany|frankfurt|\bde\b/i },
  { flag: FR, pattern: /法国|法國|巴黎|france|paris|\bfr\b/i },
  { flag: CA, pattern: /加拿大|canada|\bca\b/i },
  { flag: AU, pattern: /澳大利亚|澳洲|悉尼|australia|sydney|\bau\b/i },
  { flag: NL, pattern: /荷兰|netherlands|amsterdam|\bnl\b/i },
  { flag: IN, pattern: /印度|india|\bin\b/i },
  { flag: RU, pattern: /俄罗斯|russia|\bru\b/i },
  { flag: VN, pattern: /越南|vietnam|\bvn\b/i },
  { flag: CN, pattern: /中国|中國|北京|上海|广州|深圳|china|\bcn\b/i },
]

export function RegionIcon(props: { region: string | null }) {
  const Flag = regions.find((region) =>
    region.pattern.test(props.region ?? '')
  )?.flag
  if (!Flag)
    return (
      <HugeiconsIcon
        icon={Globe02Icon}
        className='text-muted-foreground size-5 shrink-0'
        aria-hidden='true'
      />
    )
  return <Flag className='w-full shrink-0 rounded-xs' aria-hidden='true' />
}
