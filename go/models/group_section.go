package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/jellydator/validation"
)

var (
	_ validation.Validatable = GroupSection("")
	_ validation.Validatable = GroupSections(nil)
	_ driver.Valuer          = GroupSections(nil)
	_ json.Marshaler         = GroupSections(nil)
)

// GroupSection names a part of the app a group admin can hide for every
// member of the group. Hiding a section removes it from the navigation, the
// dashboard, the item form and the item's actions, and stops its reminders.
// The data stays: an item that already holds data for a hidden section still
// shows it, and writes to that data are accepted.
type GroupSection string

const (
	// GroupSectionWarranties covers the warranty fields of an item, the
	// Warranties page and the warranty expiry reminders.
	GroupSectionWarranties GroupSection = "warranties"
	// GroupSectionMaintenance covers maintenance schedules, service records,
	// the Maintenance and In service pages and the maintenance reminders.
	GroupSectionMaintenance GroupSection = "maintenance"
	// GroupSectionLoans covers lending items out, the Lent page and the loan
	// return reminders.
	GroupSectionLoans GroupSection = "loans"
	// GroupSectionUpcoming covers the Upcoming page.
	GroupSectionUpcoming GroupSection = "upcoming"
)

// AllGroupSections returns every section a group can hide, in the order the
// group settings page lists them.
func AllGroupSections() []GroupSection {
	return []GroupSection{
		GroupSectionWarranties,
		GroupSectionMaintenance,
		GroupSectionLoans,
		GroupSectionUpcoming,
	}
}

// Validate implements validation.Validatable: the section must be one of
// AllGroupSections.
func (s GroupSection) Validate() error {
	if slices.Contains(AllGroupSections(), s) {
		return nil
	}
	return validation.NewError("validation_invalid_group_section",
		"must be one of: warranties, maintenance, loans, upcoming")
}

// GroupSections is the list of sections a group hides. It is stored as a
// JSON array and never as NULL: an empty list means the group hides nothing.
type GroupSections []GroupSection

// Validate implements validation.Validatable: every entry must be a known
// section. Duplicates are accepted here; Normalize drops them.
func (s GroupSections) Validate() error {
	for _, section := range s {
		if err := section.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// Contains reports whether the list holds section.
func (s GroupSections) Contains(section GroupSection) bool {
	return slices.Contains(s, section)
}

// Normalize returns the known sections of s once each, in the order of
// AllGroupSections. The result is never nil, so it encodes as [] rather than
// null.
func (s GroupSections) Normalize() GroupSections {
	out := GroupSections{}
	for _, section := range AllGroupSections() {
		if s.Contains(section) {
			out = append(out, section)
		}
	}
	return out
}

// MarshalJSON encodes a nil list as [] so clients never have to tell null
// from an empty list.
func (s GroupSections) MarshalJSON() ([]byte, error) {
	if s == nil {
		return []byte("[]"), nil
	}
	return json.Marshal([]GroupSection(s))
}

// Value implements driver.Valuer. A nil list is stored as [] because the
// column is NOT NULL.
func (s GroupSections) Value() (driver.Value, error) {
	if s == nil {
		return []byte("[]"), nil
	}
	return json.Marshal([]GroupSection(s))
}

// Scan implements sql.Scanner for the JSON column.
func (s *GroupSections) Scan(src any) error {
	var raw []byte
	switch v := src.(type) {
	case nil:
		*s = GroupSections{}
		return nil
	case []byte:
		raw = v
	case string:
		raw = []byte(v)
	default:
		return fmt.Errorf("cannot scan type %T into GroupSections", src)
	}
	var out []GroupSection
	if err := json.Unmarshal(raw, &out); err != nil {
		return err
	}
	if out == nil {
		out = []GroupSection{}
	}
	*s = out
	return nil
}
