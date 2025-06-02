package migrator

import (
	"testing"

	qt "github.com/frankban/quicktest"
)

func TestParseMigrationFileName(t *testing.T) {
	tests := []struct {
		name        string
		filename    string
		expected    *MigrationFile
		expectError bool
	}{
		{
			name:     "valid up migration",
			filename: "0000000001_create_users_table.up.sql",
			expected: &MigrationFile{
				Version:   1,
				Name:      "Create Users Table",
				Direction: "up",
				Extension: ".sql",
			},
			expectError: false,
		},
		{
			name:     "valid down migration",
			filename: "0000000002_add_email_index.down.sql",
			expected: &MigrationFile{
				Version:   2,
				Name:      "Add Email Index",
				Direction: "down",
				Extension: ".sql",
			},
			expectError: false,
		},
		{
			name:        "invalid format - no direction",
			filename:    "0000000001_create_users_table.sql",
			expected:    nil,
			expectError: true,
		},
		{
			name:        "invalid format - wrong extension",
			filename:    "0000000001_create_users_table.up.txt",
			expected:    nil,
			expectError: true,
		},
		{
			name:        "invalid format - no description",
			filename:    "0000000001_.up.sql",
			expected:    nil,
			expectError: true,
		},
		{
			name:        "invalid format - wrong version format",
			filename:    "1_create_users_table.up.sql",
			expected:    nil,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			result, err := ParseMigrationFileName(tt.filename)

			if tt.expectError {
				c.Assert(err, qt.IsNotNil)
				c.Assert(result, qt.IsNil)
			} else {
				c.Assert(err, qt.IsNil)
				c.Assert(result, qt.IsNotNil)
				c.Assert(result.Version, qt.Equals, tt.expected.Version)
				c.Assert(result.Name, qt.Equals, tt.expected.Name)
				c.Assert(result.Direction, qt.Equals, tt.expected.Direction)
				c.Assert(result.Extension, qt.Equals, tt.expected.Extension)
			}
		})
	}
}

func TestValidateMigrationFileName(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		expected bool
	}{
		{
			name:     "valid up migration",
			filename: "0000000001_create_users_table.up.sql",
			expected: true,
		},
		{
			name:     "valid down migration",
			filename: "0000000002_add_email_index.down.sql",
			expected: true,
		},
		{
			name:     "invalid format",
			filename: "invalid_filename.sql",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			result := ValidateMigrationFileName(tt.filename)
			c.Assert(result, qt.Equals, tt.expected)
		})
	}
}

func TestGenerateMigrationFileName(t *testing.T) {
	tests := []struct {
		name        string
		version     int
		description string
		direction   string
		expected    string
	}{
		{
			name:        "basic generation",
			version:     1,
			description: "Create Users Table",
			direction:   "up",
			expected:    "0000000001_create_users_table.up.sql",
		},
		{
			name:        "with special characters",
			version:     123,
			description: "Add Email Index & Constraints",
			direction:   "down",
			expected:    "0000000123_add_email_index__constraints.down.sql",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			result := GenerateMigrationFileName(tt.version, tt.description, tt.direction)
			c.Assert(result, qt.Equals, tt.expected)
		})
	}
}

func TestMigrationPair(t *testing.T) {
	c := qt.New(t)

	upFile := &MigrationFile{
		Version:   1,
		Name:      "Create Users Table",
		Direction: "up",
		Extension: ".sql",
	}

	downFile := &MigrationFile{
		Version:   1,
		Name:      "Create Users Table",
		Direction: "down",
		Extension: ".sql",
	}

	// Test complete pair
	completePair := MigrationPair{
		Up:   upFile,
		Down: downFile,
	}

	c.Assert(completePair.IsComplete(), qt.IsTrue)
	c.Assert(completePair.HasUp(), qt.IsTrue)
	c.Assert(completePair.HasDown(), qt.IsTrue)
	c.Assert(completePair.GetVersion(), qt.Equals, 1)
	c.Assert(completePair.GetDescription(), qt.Equals, "Create Users Table")

	// Test incomplete pair (only up)
	upOnlyPair := MigrationPair{
		Up:   upFile,
		Down: nil,
	}

	c.Assert(upOnlyPair.IsComplete(), qt.IsFalse)
	c.Assert(upOnlyPair.HasUp(), qt.IsTrue)
	c.Assert(upOnlyPair.HasDown(), qt.IsFalse)
	c.Assert(upOnlyPair.GetVersion(), qt.Equals, 1)
	c.Assert(upOnlyPair.GetDescription(), qt.Equals, "Create Users Table")

	// Test incomplete pair (only down)
	downOnlyPair := MigrationPair{
		Up:   nil,
		Down: downFile,
	}

	c.Assert(downOnlyPair.IsComplete(), qt.IsFalse)
	c.Assert(downOnlyPair.HasUp(), qt.IsFalse)
	c.Assert(downOnlyPair.HasDown(), qt.IsTrue)
	c.Assert(downOnlyPair.GetVersion(), qt.Equals, 1)
	c.Assert(downOnlyPair.GetDescription(), qt.Equals, "Create Users Table")

	// Test empty pair
	emptyPair := MigrationPair{}

	c.Assert(emptyPair.IsComplete(), qt.IsFalse)
	c.Assert(emptyPair.HasUp(), qt.IsFalse)
	c.Assert(emptyPair.HasDown(), qt.IsFalse)
	c.Assert(emptyPair.GetVersion(), qt.Equals, 0)
	c.Assert(emptyPair.GetDescription(), qt.Equals, "")
}

func TestGroupMigrationFiles(t *testing.T) {
	c := qt.New(t)

	files := []MigrationFile{
		{Version: 1, Name: "Create Users", Direction: "up", Extension: ".sql"},
		{Version: 1, Name: "Create Users", Direction: "down", Extension: ".sql"},
		{Version: 2, Name: "Add Index", Direction: "up", Extension: ".sql"},
		{Version: 3, Name: "Add Column", Direction: "down", Extension: ".sql"},
	}

	groups := GroupMigrationFiles(files)

	c.Assert(groups, qt.HasLen, 3)

	// Check version 1 (complete pair)
	pair1 := groups[1]
	c.Assert(pair1.IsComplete(), qt.IsTrue)
	c.Assert(pair1.GetVersion(), qt.Equals, 1)

	// Check version 2 (only up)
	pair2 := groups[2]
	c.Assert(pair2.IsComplete(), qt.IsFalse)
	c.Assert(pair2.HasUp(), qt.IsTrue)
	c.Assert(pair2.HasDown(), qt.IsFalse)

	// Check version 3 (only down)
	pair3 := groups[3]
	c.Assert(pair3.IsComplete(), qt.IsFalse)
	c.Assert(pair3.HasUp(), qt.IsFalse)
	c.Assert(pair3.HasDown(), qt.IsTrue)
}

func TestValidateMigrationPairs(t *testing.T) {
	c := qt.New(t)

	pairs := map[int]MigrationPair{
		1: {
			Up:   &MigrationFile{Version: 1, Direction: "up"},
			Down: &MigrationFile{Version: 1, Direction: "down"},
		},
		2: {
			Up:   &MigrationFile{Version: 2, Direction: "up"},
			Down: nil, // Missing down migration
		},
		3: {
			Up:   nil, // Missing up migration
			Down: &MigrationFile{Version: 3, Direction: "down"},
		},
	}

	incomplete := ValidateMigrationPairs(pairs)

	c.Assert(incomplete, qt.HasLen, 2)
	c.Assert(incomplete, qt.Contains, 2)
	c.Assert(incomplete, qt.Contains, 3)
}

func TestFindMigrationGaps(t *testing.T) {
	c := qt.New(t)

	// Test with no gaps
	versions1 := []int{1, 2, 3, 4, 5}
	gaps1 := FindMigrationGaps(versions1)
	c.Assert(gaps1, qt.HasLen, 0)

	// Test with gaps
	versions2 := []int{1, 3, 6, 8}
	gaps2 := FindMigrationGaps(versions2)
	c.Assert(gaps2, qt.HasLen, 4) // Should be 4: gaps at 2, 4, 5, 7
	c.Assert(gaps2, qt.Contains, 2)
	c.Assert(gaps2, qt.Contains, 4)
	c.Assert(gaps2, qt.Contains, 5)
	c.Assert(gaps2, qt.Contains, 7)

	// Test with empty slice
	versions3 := []int{}
	gaps3 := FindMigrationGaps(versions3)
	c.Assert(gaps3, qt.IsNil)

	// Test with single version
	versions4 := []int{1}
	gaps4 := FindMigrationGaps(versions4)
	c.Assert(gaps4, qt.HasLen, 0)
}
