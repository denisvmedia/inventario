package models

// GroupIcon describes a single entry in the curated set of emojis users can
// pick for a location group. Exported for validation, tests, and migrations;
// the frontend ships an equivalent list under
// frontend/src/constants/groupIcons.ts — both must be kept in sync.
type GroupIcon struct {
	Emoji    string
	Label    string
	Category string
}

// Category labels used by the frontend picker. Kept string-keyed so adding a
// new category only requires editing this file and its frontend sibling.
const (
	GroupIconCategoryHome    = "home"
	GroupIconCategoryWork    = "work"
	GroupIconCategoryStorage = "storage"
	GroupIconCategoryNature  = "nature"
	GroupIconCategoryTravel  = "travel"
	GroupIconCategoryHobbies = "hobbies"
	GroupIconCategoryMisc    = "misc"
)

// ValidGroupIcons is the authoritative list of acceptable group-icon values.
// An empty string is also accepted (icon is optional) — the emptiness check
// lives in the validator, not here, so callers iterating this slice see only
// real icons.
//
// Order within a category drives the order in which the frontend picker shows
// them; keep related icons adjacent. Changing an emoji here is a
// backward-incompatible change — drop a stale entry from the backend set,
// ship a data migration that clears the stale value from location_groups.icon,
// then remove it from the frontend.
var ValidGroupIcons = []GroupIcon{
	// Home & family
	{Emoji: "🏠", Label: "House", Category: GroupIconCategoryHome},
	{Emoji: "🏡", Label: "Home with garden", Category: GroupIconCategoryHome},
	{Emoji: "🏘️", Label: "Houses", Category: GroupIconCategoryHome},
	{Emoji: "🏰", Label: "Castle", Category: GroupIconCategoryHome},
	{Emoji: "🛏️", Label: "Bed", Category: GroupIconCategoryHome},
	{Emoji: "🛋️", Label: "Couch", Category: GroupIconCategoryHome},

	// Work & office
	{Emoji: "🏢", Label: "Office", Category: GroupIconCategoryWork},
	{Emoji: "🏛️", Label: "Institution", Category: GroupIconCategoryWork},
	{Emoji: "💼", Label: "Briefcase", Category: GroupIconCategoryWork},
	{Emoji: "💻", Label: "Laptop", Category: GroupIconCategoryWork},
	{Emoji: "🖨️", Label: "Printer", Category: GroupIconCategoryWork},
	{Emoji: "📇", Label: "Card index", Category: GroupIconCategoryWork},

	// Storage
	{Emoji: "📦", Label: "Box", Category: GroupIconCategoryStorage},
	{Emoji: "🗄️", Label: "Filing cabinet", Category: GroupIconCategoryStorage},
	{Emoji: "🗃️", Label: "Card file", Category: GroupIconCategoryStorage},
	{Emoji: "🗂️", Label: "Dividers", Category: GroupIconCategoryStorage},
	{Emoji: "📁", Label: "Folder", Category: GroupIconCategoryStorage},
	{Emoji: "🧰", Label: "Toolbox", Category: GroupIconCategoryStorage},

	// Nature
	{Emoji: "🌳", Label: "Tree", Category: GroupIconCategoryNature},
	{Emoji: "🌲", Label: "Evergreen", Category: GroupIconCategoryNature},
	{Emoji: "🌵", Label: "Cactus", Category: GroupIconCategoryNature},
	{Emoji: "🌷", Label: "Tulip", Category: GroupIconCategoryNature},
	{Emoji: "🍀", Label: "Four-leaf clover", Category: GroupIconCategoryNature},
	{Emoji: "⛰️", Label: "Mountain", Category: GroupIconCategoryNature},

	// Travel & transport
	{Emoji: "🚗", Label: "Car", Category: GroupIconCategoryTravel},
	{Emoji: "🚚", Label: "Truck", Category: GroupIconCategoryTravel},
	{Emoji: "🚲", Label: "Bicycle", Category: GroupIconCategoryTravel},
	{Emoji: "✈️", Label: "Plane", Category: GroupIconCategoryTravel},
	{Emoji: "🧳", Label: "Luggage", Category: GroupIconCategoryTravel},
	{Emoji: "⛵", Label: "Sailboat", Category: GroupIconCategoryTravel},

	// Hobbies
	{Emoji: "🎨", Label: "Art", Category: GroupIconCategoryHobbies},
	{Emoji: "🎸", Label: "Guitar", Category: GroupIconCategoryHobbies},
	{Emoji: "🎮", Label: "Video game", Category: GroupIconCategoryHobbies},
	{Emoji: "📚", Label: "Books", Category: GroupIconCategoryHobbies},
	{Emoji: "📷", Label: "Camera", Category: GroupIconCategoryHobbies},
	{Emoji: "⚽", Label: "Football", Category: GroupIconCategoryHobbies},
	{Emoji: "🎲", Label: "Dice", Category: GroupIconCategoryHobbies},
	{Emoji: "🧩", Label: "Jigsaw", Category: GroupIconCategoryHobbies},

	// Misc
	{Emoji: "⭐", Label: "Star", Category: GroupIconCategoryMisc},
	{Emoji: "💎", Label: "Gem", Category: GroupIconCategoryMisc},
	{Emoji: "🔥", Label: "Fire", Category: GroupIconCategoryMisc},
	{Emoji: "⚙️", Label: "Gear", Category: GroupIconCategoryMisc},
	{Emoji: "🛠️", Label: "Tools", Category: GroupIconCategoryMisc},
	{Emoji: "🎯", Label: "Target", Category: GroupIconCategoryMisc},
	{Emoji: "🏷️", Label: "Label", Category: GroupIconCategoryMisc},
	{Emoji: "💡", Label: "Lightbulb", Category: GroupIconCategoryMisc},
}

// validGroupIconSet is a lookup table built from ValidGroupIcons once, at
// package init, so per-request validation stays O(1). Not exported — callers
// that need membership should use IsValidGroupIcon.
var validGroupIconSet = func() map[string]struct{} {
	set := make(map[string]struct{}, len(ValidGroupIcons))
	for _, ic := range ValidGroupIcons {
		set[ic.Emoji] = struct{}{}
	}
	return set
}()

// IsValidGroupIcon reports whether the given string is one of the curated
// group icons. An empty string is NOT considered valid here — the empty /
// unset case must be handled separately by the caller (e.g. the request
// validator allows empty, storage accepts empty). This keeps the function
// useful for both validation (reject nonsense) and data-migration filtering
// (rewrite anything-not-in-the-set to empty).
func IsValidGroupIcon(icon string) bool {
	if icon == "" {
		return false
	}
	_, ok := validGroupIconSet[icon]
	return ok
}
