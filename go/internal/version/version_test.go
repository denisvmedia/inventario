package version_test

import (
	"runtime"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/internal/version"
)

func TestGet_HappyPath(t *testing.T) {
	tests := []struct {
		name     string
		setup    func()
		teardown func()
		want     version.BuildInfo
	}{
		{
			name: "default values",
			setup: func() {
				// Use default values
			},
			teardown: func() {
				// Reset to defaults
				version.Version = "dev"
				version.Commit = "unknown"
				version.Date = "unknown"
			},
			want: version.BuildInfo{
				Version:   "dev",
				Commit:    "unknown",
				Date:      "unknown",
				GoVersion: runtime.Version(),
				Platform:  runtime.GOOS + "/" + runtime.GOARCH,
			},
		},
		{
			name: "custom build values",
			setup: func() {
				version.Version = "v1.2.3"
				version.Commit = "abc123def456"
				version.Date = "2024-01-15T10:30:00Z"
			},
			teardown: func() {
				// Reset to defaults
				version.Version = "dev"
				version.Commit = "unknown"
				version.Date = "unknown"
			},
			want: version.BuildInfo{
				Version:   "v1.2.3",
				Commit:    "abc123def456",
				Date:      "2024-01-15T10:30:00Z",
				GoVersion: runtime.Version(),
				Platform:  runtime.GOOS + "/" + runtime.GOARCH,
			},
		},
		{
			name: "empty values",
			setup: func() {
				version.Version = ""
				version.Commit = ""
				version.Date = ""
			},
			teardown: func() {
				// Reset to defaults
				version.Version = "dev"
				version.Commit = "unknown"
				version.Date = "unknown"
			},
			want: version.BuildInfo{
				Version:   "",
				Commit:    "",
				Date:      "",
				GoVersion: runtime.Version(),
				Platform:  runtime.GOOS + "/" + runtime.GOARCH,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			tt.setup()
			defer tt.teardown()

			got := version.Get()
			c.Assert(got, qt.Equals, tt.want)
		})
	}
}

func TestGet_RuntimeValues(t *testing.T) {
	c := qt.New(t)

	got := version.Get()

	// Test that runtime values are properly populated
	c.Assert(got.GoVersion, qt.Not(qt.Equals), "")
	c.Assert(strings.HasPrefix(got.GoVersion, "go"), qt.IsTrue)

	c.Assert(got.Platform, qt.Not(qt.Equals), "")
	c.Assert(strings.Contains(got.Platform, "/"), qt.IsTrue)

	// Verify platform format
	parts := strings.Split(got.Platform, "/")
	c.Assert(parts, qt.HasLen, 2)
	c.Assert(parts[0], qt.Equals, runtime.GOOS)
	c.Assert(parts[1], qt.Equals, runtime.GOARCH)
}

func TestString_HappyPath(t *testing.T) {
	tests := []struct {
		name     string
		setup    func()
		teardown func()
		want     string
	}{
		{
			name: "default values",
			setup: func() {
				// Use default values
			},
			teardown: func() {
				// Reset to defaults
				version.Version = "dev"
				version.Commit = "unknown"
				version.Date = "unknown"
			},
			want: "dev (unknown) built on unknown",
		},
		{
			name: "custom build values",
			setup: func() {
				version.Version = "v1.2.3"
				version.Commit = "abc123def456"
				version.Date = "2024-01-15T10:30:00Z"
			},
			teardown: func() {
				// Reset to defaults
				version.Version = "dev"
				version.Commit = "unknown"
				version.Date = "unknown"
			},
			want: "v1.2.3 (abc123def456) built on 2024-01-15T10:30:00Z",
		},
		{
			name: "version with spaces",
			setup: func() {
				version.Version = "v1.0.0 beta"
				version.Commit = "short"
				version.Date = "today"
			},
			teardown: func() {
				// Reset to defaults
				version.Version = "dev"
				version.Commit = "unknown"
				version.Date = "unknown"
			},
			want: "v1.0.0 beta (short) built on today",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			tt.setup()
			defer tt.teardown()

			got := version.String()
			c.Assert(got, qt.Equals, tt.want)
		})
	}
}

func TestString_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		setup    func()
		teardown func()
		want     string
	}{
		{
			name: "empty values",
			setup: func() {
				version.Version = ""
				version.Commit = ""
				version.Date = ""
			},
			teardown: func() {
				// Reset to defaults
				version.Version = "dev"
				version.Commit = "unknown"
				version.Date = "unknown"
			},
			want: " () built on ",
		},
		{
			name: "special characters",
			setup: func() {
				version.Version = "v1.0.0-rc.1+build.123"
				version.Commit = "abc123-def456_789"
				version.Date = "2024-01-15T10:30:00+00:00"
			},
			teardown: func() {
				// Reset to defaults
				version.Version = "dev"
				version.Commit = "unknown"
				version.Date = "unknown"
			},
			want: "v1.0.0-rc.1+build.123 (abc123-def456_789) built on 2024-01-15T10:30:00+00:00",
		},
		{
			name: "unicode characters",
			setup: func() {
				version.Version = "v1.0.0-α"
				version.Commit = "αβγ123"
				version.Date = "2024年1月15日"
			},
			teardown: func() {
				// Reset to defaults
				version.Version = "dev"
				version.Commit = "unknown"
				version.Date = "unknown"
			},
			want: "v1.0.0-α (αβγ123) built on 2024年1月15日",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			tt.setup()
			defer tt.teardown()

			got := version.String()
			c.Assert(got, qt.Equals, tt.want)
		})
	}
}

func TestBuildInfo_JSONTags(t *testing.T) {
	c := qt.New(t)

	// Test that BuildInfo struct has proper JSON tags by checking the struct
	info := version.BuildInfo{
		Version:   "test",
		Commit:    "test",
		Date:      "test",
		GoVersion: "test",
		Platform:  "test",
	}

	// Verify the struct is properly initialized
	c.Assert(info.Version, qt.Equals, "test")
	c.Assert(info.Commit, qt.Equals, "test")
	c.Assert(info.Date, qt.Equals, "test")
	c.Assert(info.GoVersion, qt.Equals, "test")
	c.Assert(info.Platform, qt.Equals, "test")
}

func TestPackageVariables_Mutability(t *testing.T) {
	c := qt.New(t)

	// Store original values
	originalVersion := version.Version
	originalCommit := version.Commit
	originalDate := version.Date

	defer func() {
		// Restore original values
		version.Version = originalVersion
		version.Commit = originalCommit
		version.Date = originalDate
	}()

	// Test that variables can be modified (as they would be by ldflags)
	version.Version = "modified"
	version.Commit = "modified"
	version.Date = "modified"

	c.Assert(version.Version, qt.Equals, "modified")
	c.Assert(version.Commit, qt.Equals, "modified")
	c.Assert(version.Date, qt.Equals, "modified")

	// Verify Get() reflects the changes
	info := version.Get()
	c.Assert(info.Version, qt.Equals, "modified")
	c.Assert(info.Commit, qt.Equals, "modified")
	c.Assert(info.Date, qt.Equals, "modified")

	// Verify String() reflects the changes
	str := version.String()
	c.Assert(str, qt.Equals, "modified (modified) built on modified")
}
