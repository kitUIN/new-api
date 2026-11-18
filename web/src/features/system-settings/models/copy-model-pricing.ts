import { z } from 'zod'

export type PricingMaps = Record<string, Record<string, number | string>>

export function copyModelPricingSchema(maps: PricingMaps) {
  return z.object({
    name: z
      .string()
      .trim()
      .min(1, 'Model name is required')
      .refine(
        (name) => !Object.values(maps).some((map) => Object.hasOwn(map, name)),
        'Model name already exists'
      ),
  })
}

export function copyModelPricing(
  maps: PricingMaps,
  sourceName: string,
  targetName: string
): Record<string, string> {
  return Object.fromEntries(
    Object.entries(maps)
      .filter(([, map]) => Object.hasOwn(map, sourceName))
      .map(([key, map]) => [
        key,
        JSON.stringify({ ...map, [targetName]: map[sourceName] }, null, 2),
      ])
  )
}
