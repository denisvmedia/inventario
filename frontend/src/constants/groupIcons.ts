/**
 * Curated set of emoji icons a user can pick for a location group.
 *
 * Mirror of go/models/group_icons.go — the backend rejects any value that
 * isn't in this list (plus empty) with a 422. Both sides must stay in sync:
 * adding an emoji is backward-compatible, but removing/renaming one requires
 * a data migration clearing the stale value from location_groups.icon.
 *
 * Order within a category drives the picker grid order; keep related icons
 * adjacent. Issue #1255.
 */

export const GROUP_ICON_CATEGORY_HOME = 'home'
export const GROUP_ICON_CATEGORY_WORK = 'work'
export const GROUP_ICON_CATEGORY_STORAGE = 'storage'
export const GROUP_ICON_CATEGORY_NATURE = 'nature'
export const GROUP_ICON_CATEGORY_TRAVEL = 'travel'
export const GROUP_ICON_CATEGORY_HOBBIES = 'hobbies'
export const GROUP_ICON_CATEGORY_MISC = 'misc'

export interface GroupIcon {
  emoji: string
  label: string
  category: string
}

export interface GroupIconCategory {
  id: string
  label: string
}

export const GROUP_ICON_CATEGORIES: GroupIconCategory[] = [
  { id: GROUP_ICON_CATEGORY_HOME, label: 'Home' },
  { id: GROUP_ICON_CATEGORY_WORK, label: 'Work' },
  { id: GROUP_ICON_CATEGORY_STORAGE, label: 'Storage' },
  { id: GROUP_ICON_CATEGORY_NATURE, label: 'Nature' },
  { id: GROUP_ICON_CATEGORY_TRAVEL, label: 'Travel' },
  { id: GROUP_ICON_CATEGORY_HOBBIES, label: 'Hobbies' },
  { id: GROUP_ICON_CATEGORY_MISC, label: 'Misc' },
]

export const GROUP_ICONS: GroupIcon[] = [
  // Home & family
  { emoji: '🏠', label: 'House', category: GROUP_ICON_CATEGORY_HOME },
  { emoji: '🏡', label: 'Home with garden', category: GROUP_ICON_CATEGORY_HOME },
  { emoji: '🏘️', label: 'Houses', category: GROUP_ICON_CATEGORY_HOME },
  { emoji: '🏰', label: 'Castle', category: GROUP_ICON_CATEGORY_HOME },
  { emoji: '🛏️', label: 'Bed', category: GROUP_ICON_CATEGORY_HOME },
  { emoji: '🛋️', label: 'Couch', category: GROUP_ICON_CATEGORY_HOME },

  // Work & office
  { emoji: '🏢', label: 'Office', category: GROUP_ICON_CATEGORY_WORK },
  { emoji: '🏛️', label: 'Institution', category: GROUP_ICON_CATEGORY_WORK },
  { emoji: '💼', label: 'Briefcase', category: GROUP_ICON_CATEGORY_WORK },
  { emoji: '💻', label: 'Laptop', category: GROUP_ICON_CATEGORY_WORK },
  { emoji: '🖨️', label: 'Printer', category: GROUP_ICON_CATEGORY_WORK },
  { emoji: '📇', label: 'Card index', category: GROUP_ICON_CATEGORY_WORK },

  // Storage
  { emoji: '📦', label: 'Box', category: GROUP_ICON_CATEGORY_STORAGE },
  { emoji: '🗄️', label: 'Filing cabinet', category: GROUP_ICON_CATEGORY_STORAGE },
  { emoji: '🗃️', label: 'Card file', category: GROUP_ICON_CATEGORY_STORAGE },
  { emoji: '🗂️', label: 'Dividers', category: GROUP_ICON_CATEGORY_STORAGE },
  { emoji: '📁', label: 'Folder', category: GROUP_ICON_CATEGORY_STORAGE },
  { emoji: '🧰', label: 'Toolbox', category: GROUP_ICON_CATEGORY_STORAGE },

  // Nature
  { emoji: '🌳', label: 'Tree', category: GROUP_ICON_CATEGORY_NATURE },
  { emoji: '🌲', label: 'Evergreen', category: GROUP_ICON_CATEGORY_NATURE },
  { emoji: '🌵', label: 'Cactus', category: GROUP_ICON_CATEGORY_NATURE },
  { emoji: '🌷', label: 'Tulip', category: GROUP_ICON_CATEGORY_NATURE },
  { emoji: '🍀', label: 'Four-leaf clover', category: GROUP_ICON_CATEGORY_NATURE },
  { emoji: '⛰️', label: 'Mountain', category: GROUP_ICON_CATEGORY_NATURE },

  // Travel & transport
  { emoji: '🚗', label: 'Car', category: GROUP_ICON_CATEGORY_TRAVEL },
  { emoji: '🚚', label: 'Truck', category: GROUP_ICON_CATEGORY_TRAVEL },
  { emoji: '🚲', label: 'Bicycle', category: GROUP_ICON_CATEGORY_TRAVEL },
  { emoji: '✈️', label: 'Plane', category: GROUP_ICON_CATEGORY_TRAVEL },
  { emoji: '🧳', label: 'Luggage', category: GROUP_ICON_CATEGORY_TRAVEL },
  { emoji: '⛵', label: 'Sailboat', category: GROUP_ICON_CATEGORY_TRAVEL },

  // Hobbies
  { emoji: '🎨', label: 'Art', category: GROUP_ICON_CATEGORY_HOBBIES },
  { emoji: '🎸', label: 'Guitar', category: GROUP_ICON_CATEGORY_HOBBIES },
  { emoji: '🎮', label: 'Video game', category: GROUP_ICON_CATEGORY_HOBBIES },
  { emoji: '📚', label: 'Books', category: GROUP_ICON_CATEGORY_HOBBIES },
  { emoji: '📷', label: 'Camera', category: GROUP_ICON_CATEGORY_HOBBIES },
  { emoji: '⚽', label: 'Football', category: GROUP_ICON_CATEGORY_HOBBIES },
  { emoji: '🎲', label: 'Dice', category: GROUP_ICON_CATEGORY_HOBBIES },
  { emoji: '🧩', label: 'Jigsaw', category: GROUP_ICON_CATEGORY_HOBBIES },

  // Misc
  { emoji: '⭐', label: 'Star', category: GROUP_ICON_CATEGORY_MISC },
  { emoji: '💎', label: 'Gem', category: GROUP_ICON_CATEGORY_MISC },
  { emoji: '🔥', label: 'Fire', category: GROUP_ICON_CATEGORY_MISC },
  { emoji: '⚙️', label: 'Gear', category: GROUP_ICON_CATEGORY_MISC },
  { emoji: '🛠️', label: 'Tools', category: GROUP_ICON_CATEGORY_MISC },
  { emoji: '🎯', label: 'Target', category: GROUP_ICON_CATEGORY_MISC },
  { emoji: '🏷️', label: 'Label', category: GROUP_ICON_CATEGORY_MISC },
  { emoji: '💡', label: 'Lightbulb', category: GROUP_ICON_CATEGORY_MISC },
  { emoji: '🛡️', label: 'Shield', category: GROUP_ICON_CATEGORY_MISC },
  { emoji: '🔒', label: 'Lock', category: GROUP_ICON_CATEGORY_MISC },
  { emoji: '🧪', label: 'Lab', category: GROUP_ICON_CATEGORY_MISC },
  { emoji: '🧷', label: 'Safety pin', category: GROUP_ICON_CATEGORY_MISC },
]

const GROUP_ICON_SET = new Set(GROUP_ICONS.map((ic) => ic.emoji))

/**
 * Whether the given value is one of the curated group icons. An empty string
 * is NOT considered valid — callers that accept "unset" must handle that case
 * themselves.
 */
export function isValidGroupIcon(value: string): boolean {
  if (!value) return false
  return GROUP_ICON_SET.has(value)
}
