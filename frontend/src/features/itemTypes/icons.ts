// The icons an item type can carry (#2802). The server stores the key (a
// lowercase, hyphenated name, see itemTypeIconPattern in
// go/models/item_type.go) and the app maps it to a Lucide icon. A key the
// app does not know shows the generic package icon, so a type created by a
// newer app version still renders.
import {
  Armchair,
  Baby,
  Bed,
  Bike,
  BookOpen,
  Briefcase,
  Camera,
  Car,
  CookingPot,
  Drill,
  Dumbbell,
  Gamepad2,
  Gem,
  Gift,
  Glasses,
  Guitar,
  Hammer,
  Headphones,
  Lamp,
  Laptop,
  Music,
  Package,
  Palette,
  PawPrint,
  Plane,
  Refrigerator,
  Sailboat,
  Shirt,
  Smartphone,
  Sofa,
  Sprout,
  Tent,
  ToyBrick,
  Tv,
  WashingMachine,
  Watch,
  Wine,
  Wrench,
  type LucideIcon,
} from "lucide-react"

// The icon picker shows the keys in this order: the built-in types' icons
// first, then the rest grouped loosely by theme.
export const ITEM_TYPE_ICONS = {
  refrigerator: Refrigerator,
  laptop: Laptop,
  wrench: Wrench,
  armchair: Armchair,
  shirt: Shirt,
  package: Package,
  car: Car,
  bike: Bike,
  sailboat: Sailboat,
  plane: Plane,
  smartphone: Smartphone,
  tv: Tv,
  camera: Camera,
  headphones: Headphones,
  "gamepad-2": Gamepad2,
  watch: Watch,
  gem: Gem,
  glasses: Glasses,
  "book-open": BookOpen,
  music: Music,
  guitar: Guitar,
  palette: Palette,
  hammer: Hammer,
  drill: Drill,
  sofa: Sofa,
  bed: Bed,
  lamp: Lamp,
  "washing-machine": WashingMachine,
  "cooking-pot": CookingPot,
  wine: Wine,
  dumbbell: Dumbbell,
  tent: Tent,
  "toy-brick": ToyBrick,
  baby: Baby,
  "paw-print": PawPrint,
  sprout: Sprout,
  briefcase: Briefcase,
  gift: Gift,
} as const satisfies Record<string, LucideIcon>

export type ItemTypeIconKey = keyof typeof ITEM_TYPE_ICONS

export const ITEM_TYPE_ICON_KEYS = Object.keys(ITEM_TYPE_ICONS) as ItemTypeIconKey[]

// The icon of a type without one, and of a key the app does not know.
export const ITEM_TYPE_FALLBACK_ICON: LucideIcon = Package

// The icon a new type starts with in the form.
export const DEFAULT_ITEM_TYPE_ICON: ItemTypeIconKey = "package"

export function isItemTypeIconKey(key: string | undefined | null): key is ItemTypeIconKey {
  return !!key && Object.hasOwn(ITEM_TYPE_ICONS, key)
}

// ITEM_TYPE_ICON_LOOKUP maps any key to its icon, or undefined. It has no
// prototype, so a key such as "constructor" finds nothing. A component
// that renders the icon looks it up here directly, `LOOKUP[key] ??
// ITEM_TYPE_FALLBACK_ICON`, rather than through a function: the React
// lint rules treat a component returned by a call during render as one
// created during render.
export const ITEM_TYPE_ICON_LOOKUP: Readonly<Record<string, LucideIcon | undefined>> =
  Object.assign(Object.create(null) as Record<string, LucideIcon | undefined>, ITEM_TYPE_ICONS)

export function itemTypeIcon(key: string | undefined | null): LucideIcon {
  return (key ? ITEM_TYPE_ICON_LOOKUP[key] : undefined) ?? ITEM_TYPE_FALLBACK_ICON
}
