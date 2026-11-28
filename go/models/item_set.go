package models

import (
	"regexp"
	"strings"
)

// ItemSet names a ready-made set of item types and custom fields a group
// admin can add to a group and remove again (#1607). Adding a set creates
// the types and fields the group does not have yet and shows the fields on
// the set's types; removing it deletes what the set created and nothing
// uses. A set is not a kind of group: it adds rows a member could have
// created by hand, and every one of them can be renamed or deleted like
// any other.
type ItemSet string

// ItemSetCollection is the set for collections: art, books, coins, stamps,
// records, watches and cards, with condition, catalog reference, edition,
// signature, provenance, certificate and appraisal fields.
const ItemSetCollection ItemSet = "collection"

// itemSetEntryKeyPattern is the shape of the key of one entry of a set.
var itemSetEntryKeyPattern = regexp.MustCompile(`^[a-z0-9]+(?:_[a-z0-9]+)*$`)

// ItemSetEntry names the entry of an item set that created an item type or
// a custom field, as "<set>/<key>" ("collection/coins"). The rows a member
// creates have none. The registries keep it as stored on every update, so
// a renamed type is still known as the set's. It is what removing the set
// goes by: a type or field without it is never deleted with the set.
type ItemSetEntry string

// NewItemSetEntry returns the entry key of set names.
func NewItemSetEntry(set ItemSet, key string) ItemSetEntry {
	return ItemSetEntry(string(set) + "/" + key)
}

// Set returns the set the entry belongs to, or "" for no entry.
func (e ItemSetEntry) Set() ItemSet {
	set, _, _ := strings.Cut(string(e), "/")
	return ItemSet(set)
}

// IsValid reports whether e names an entry of a known set. The key is not
// checked against the set's definition, so a row created by a later
// version of a set still counts as the set's.
func (e ItemSetEntry) IsValid() bool {
	set, key, ok := strings.Cut(string(e), "/")
	if !ok {
		return false
	}
	if _, known := ItemSetDefinitionFor(ItemSet(set)); !known {
		return false
	}
	return itemSetEntryKeyPattern.MatchString(key)
}

// ItemSetItemType is an item type a set creates. Name is English: a set
// creates rows with stored names, which members rename in their own
// language as they would a type they added themselves.
type ItemSetItemType struct {
	Key  string
	Name string
	Icon string
}

// ItemSetField is a custom field a set creates. A select field's options
// carry fixed IDs, so the same set gives the same options in every group.
type ItemSetField struct {
	Key     string
	Name    string
	Type    CustomFieldType
	Options []CustomFieldOption
}

// ItemSetDefinition is what a set adds: its item types, and its custom
// fields, each shown on the template of every type of the set.
type ItemSetDefinition struct {
	Set       ItemSet
	ItemTypes []ItemSetItemType
	Fields    []ItemSetField
}

// Entry returns the entry key of an item type or field of the set.
func (d ItemSetDefinition) Entry(key string) ItemSetEntry {
	return NewItemSetEntry(d.Set, key)
}

// CollectionSet is the definition of ItemSetCollection. Serial number stays
// the built-in field it is; the set adds Catalog reference next to it
// rather than relabeling it.
var CollectionSet = ItemSetDefinition{
	Set: ItemSetCollection,
	ItemTypes: []ItemSetItemType{
		{Key: "art", Name: "Art", Icon: "palette"},
		{Key: "books", Name: "Books", Icon: "book-open"},
		{Key: "coins", Name: "Coins", Icon: "coins"},
		{Key: "stamps", Name: "Stamps", Icon: "stamp"},
		{Key: "records", Name: "Records", Icon: "disc-3"},
		{Key: "watches", Name: "Watches", Icon: "watch"},
		{Key: "cards", Name: "Cards", Icon: "layers"},
	},
	Fields: []ItemSetField{
		{Key: "condition", Name: "Condition", Type: CustomFieldTypeSelect, Options: []CustomFieldOption{
			{ID: "mint", Label: "Mint"},
			{ID: "verygood", Label: "Very good"},
			{ID: "good", Label: "Good"},
			{ID: "fair", Label: "Fair"},
			{ID: "poor", Label: "Poor"},
		}},
		{Key: "catalog_reference", Name: "Catalog reference", Type: CustomFieldTypeText},
		{Key: "edition", Name: "Edition or variant", Type: CustomFieldTypeText},
		{Key: "signed", Name: "Signed", Type: CustomFieldTypeBoolean},
		{Key: "provenance", Name: "Provenance", Type: CustomFieldTypeLongText},
		{Key: "certificate", Name: "Certificate of authenticity", Type: CustomFieldTypeBoolean},
		{Key: "appraised_value", Name: "Appraised value", Type: CustomFieldTypeMoney},
		{Key: "appraisal_date", Name: "Appraisal date", Type: CustomFieldTypeDate},
	},
}

// ItemSetDefinitionFor returns the definition of set.
func ItemSetDefinitionFor(set ItemSet) (ItemSetDefinition, bool) {
	if set == ItemSetCollection {
		return CollectionSet, true
	}
	return ItemSetDefinition{}, false
}
