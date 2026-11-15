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
interface GroupHierarchyItem {
  group: string
  isCombinationGroup?: boolean
  memberGroups?: string[]
}

interface GroupStatsRow<T> {
  key: string
  depth: 0 | 1
  stats: T
}

// Combination groups cannot be nested. Keep their member rows in the flat
// display list so collapsing model/user details never hides member groups.
export function buildGroupStatsRows<T extends GroupHierarchyItem>(
  groups: T[]
): GroupStatsRow<T>[] {
  const byGroup = new Map(groups.map((group) => [group.group, group]))
  const membersByGroup = new Map<string, T[]>()
  const childGroups = new Set<string>()

  for (const group of groups) {
    if (!group.isCombinationGroup) continue

    const members: T[] = []
    for (const name of new Set(group.memberGroups)) {
      const member = byGroup.get(name)
      if (!member || member.isCombinationGroup) continue
      members.push(member)
      childGroups.add(name)
    }
    membersByGroup.set(group.group, members)
  }

  const rows: GroupStatsRow<T>[] = []
  for (const group of groups) {
    if (childGroups.has(group.group)) continue

    rows.push({ key: JSON.stringify([group.group]), depth: 0, stats: group })
    for (const member of membersByGroup.get(group.group) ?? []) {
      rows.push({
        key: JSON.stringify([group.group, member.group]),
        depth: 1,
        stats: member,
      })
    }
  }
  return rows
}
