// Curated list of emoji icons a group can pick. Mirrors
// go/models/group_icons.go and frontend/src/constants/groupIcons.ts —
// the backend rejects any value not in this list with 422. Keep all
// three in sync; adding an emoji is backward-compatible, but
// removing/renaming one needs a data migration. See #1255.

export const GROUP_ICON_CATEGORIES = [
  "home",
  "work",
  "storage",
  "nature",
  "travel",
  "hobbies",
  "misc",
] as const
export type GroupIconCategory = (typeof GROUP_ICON_CATEGORIES)[number]

export interface GroupIcon {
  emoji: string
  // Plain English label — surfaced as the button's accessible name in
  // the icon picker. cs/ru locales reuse the same emoji and add their
  // own labels via i18n if/when the picker grows that surface.
  label: string
  category: GroupIconCategory
}

export const GROUP_ICONS: GroupIcon[] = [
  // Home & family.
  { emoji: "🏠", label: "House", category: "home" },
  { emoji: "🏡", label: "Home with garden", category: "home" },
  { emoji: "🏘️", label: "Houses", category: "home" },
  { emoji: "🏰", label: "Castle", category: "home" },
  { emoji: "🛏️", label: "Bed", category: "home" },
  { emoji: "🛋️", label: "Couch", category: "home" },
  // Work & office.
  { emoji: "🏢", label: "Office", category: "work" },
  { emoji: "🏛️", label: "Institution", category: "work" },
  { emoji: "💼", label: "Briefcase", category: "work" },
  { emoji: "💻", label: "Laptop", category: "work" },
  { emoji: "🖨️", label: "Printer", category: "work" },
  { emoji: "📇", label: "Card index", category: "work" },
  // Storage.
  { emoji: "📦", label: "Box", category: "storage" },
  { emoji: "🗄️", label: "Filing cabinet", category: "storage" },
  { emoji: "🗃️", label: "Card file", category: "storage" },
  { emoji: "🗂️", label: "Dividers", category: "storage" },
  { emoji: "📁", label: "Folder", category: "storage" },
  { emoji: "🧰", label: "Toolbox", category: "storage" },
  // Nature.
  { emoji: "🌳", label: "Tree", category: "nature" },
  { emoji: "🌲", label: "Evergreen", category: "nature" },
  { emoji: "🌵", label: "Cactus", category: "nature" },
  { emoji: "🌷", label: "Tulip", category: "nature" },
  { emoji: "🍀", label: "Four-leaf clover", category: "nature" },
  { emoji: "⛰️", label: "Mountain", category: "nature" },
  // Travel & transport.
  { emoji: "🚗", label: "Car", category: "travel" },
  { emoji: "🚚", label: "Truck", category: "travel" },
  { emoji: "🚲", label: "Bicycle", category: "travel" },
  { emoji: "✈️", label: "Plane", category: "travel" },
  { emoji: "🧳", label: "Luggage", category: "travel" },
  { emoji: "⛵", label: "Sailboat", category: "travel" },
  // Hobbies.
  { emoji: "🎨", label: "Art", category: "hobbies" },
  { emoji: "🎸", label: "Guitar", category: "hobbies" },
  { emoji: "🎮", label: "Video game", category: "hobbies" },
  { emoji: "📚", label: "Books", category: "hobbies" },
  { emoji: "📷", label: "Camera", category: "hobbies" },
  { emoji: "⚽", label: "Football", category: "hobbies" },
  { emoji: "🎲", label: "Dice", category: "hobbies" },
  { emoji: "🧩", label: "Jigsaw", category: "hobbies" },
  // Misc.
  { emoji: "⭐", label: "Star", category: "misc" },
  { emoji: "💎", label: "Gem", category: "misc" },
  { emoji: "🔥", label: "Fire", category: "misc" },
  { emoji: "⚙️", label: "Gear", category: "misc" },
  { emoji: "🛠️", label: "Tools", category: "misc" },
  { emoji: "🎯", label: "Target", category: "misc" },
  { emoji: "🏷️", label: "Label", category: "misc" },
]

const ALLOWED = new Set(GROUP_ICONS.map((g) => g.emoji))

export function isAllowedGroupIcon(value: string): boolean {
  return value === "" || ALLOWED.has(value)
}
