import type { TFunction } from "i18next"
import type { LucideIcon } from "lucide-react"

import { nameKey } from "@/lib/name-key"

import type { ItemType } from "./api"
import { ITEM_TYPE_ICON_LOOKUP, itemTypeIcon } from "./icons"

// The slugs of the types every group starts with, in their seed order
// (models.BuiltinItemTypes). Only these have translated default names.
export const BUILTIN_ITEM_TYPE_SLUGS = [
  "white_goods",
  "electronics",
  "equipment",
  "furniture",
  "clothes",
  "other",
] as const

export type BuiltinItemTypeSlug = (typeof BUILTIN_ITEM_TYPE_SLUGS)[number]

// The English default names and the icons of the built-in types, as the
// server seeds them. The server checks a new name against the English
// default of a built-in type that was not renamed.
const BUILTIN_DEFAULTS: Record<BuiltinItemTypeSlug, { name: string; icon: string }> = {
  white_goods: { name: "Appliance", icon: "refrigerator" },
  electronics: { name: "Electronics", icon: "laptop" },
  equipment: { name: "Equipment", icon: "wrench" },
  furniture: { name: "Furniture", icon: "armchair" },
  clothes: { name: "Clothes", icon: "shirt" },
  other: { name: "Other", icon: "package" },
}

// BUILTIN_ITEM_TYPE_ICON_LOOKUP maps a built-in slug to its default icon,
// and any other key to undefined (see ITEM_TYPE_ICON_LOOKUP).
export const BUILTIN_ITEM_TYPE_ICON_LOOKUP: Readonly<Record<string, LucideIcon | undefined>> =
  Object.assign(
    Object.create(null) as Record<string, LucideIcon | undefined>,
    Object.fromEntries(
      Object.entries(BUILTIN_DEFAULTS).map(([slug, { icon }]) => [
        slug,
        ITEM_TYPE_ICON_LOOKUP[icon],
      ])
    )
  )

// builtinItemTypes are the types a group starts with, as the server seeds
// them. The app offers them where there is no group to ask, such as the
// item form of a visitor who has not signed up yet; the server seeds the
// same six in the group the item is saved to.
export function builtinItemTypes(): ItemType[] {
  return BUILTIN_ITEM_TYPE_SLUGS.map((slug) => ({
    id: `builtin-${slug}`,
    slug,
    name: "",
    icon: BUILTIN_DEFAULTS[slug].icon,
    builtin: true,
    item_count: 0,
    trashed_item_count: 0,
  }))
}

export function isBuiltinItemTypeSlug(
  slug: string | undefined | null
): slug is BuiltinItemTypeSlug {
  return !!slug && Object.hasOwn(BUILTIN_DEFAULTS, slug)
}

type Named = Pick<ItemType, "slug" | "name">

// builtinItemTypeLabel is the translated default name of a built-in type.
export function builtinItemTypeLabel(t: TFunction, slug: BuiltinItemTypeSlug): string {
  return t(`commodities:type.${slug}`)
}

// itemTypeLabel is the name a type goes by in the app: the name a member
// gave it, the translated default of a built-in type, or the slug of a
// type the group no longer has.
export function itemTypeLabel(t: TFunction, type: Named): string {
  if (type.name) return type.name
  const slug = type.slug ?? ""
  return isBuiltinItemTypeSlug(slug) ? builtinItemTypeLabel(t, slug) : slug
}

// itemTypeIconFor is the icon of a type, or of a built-in slug when the
// type itself is not at hand.
export function itemTypeIconFor(
  type: Pick<ItemType, "icon"> | undefined,
  slug?: string | null
): LucideIcon {
  if (type) return itemTypeIcon(type.icon)
  return itemTypeIcon(isBuiltinItemTypeSlug(slug) ? BUILTIN_DEFAULTS[slug].icon : undefined)
}

// serverNameKey is the key the server's uniqueness check gives the type.
function serverNameKey(type: Named): string {
  if (type.name) return nameKey(type.name)
  const slug = type.slug ?? ""
  return nameKey(isBuiltinItemTypeSlug(slug) ? BUILTIN_DEFAULTS[slug].name : slug)
}

// findItemTypeByName returns the type that a new type named name would
// clash with: one the server would refuse the name for, or one that shows
// under that name in the app, such as "Elektronika" for the built-in
// Electronics in Czech, which the server cannot know about.
export function findItemTypeByName(
  t: TFunction,
  types: readonly ItemType[],
  name: string
): ItemType | undefined {
  const key = nameKey(name)
  if (!key) return undefined
  return types.find(
    (type) => serverNameKey(type) === key || nameKey(itemTypeLabel(t, type)) === key
  )
}
