package services_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/services"
)

func TestThumbnailSizeNames(t *testing.T) {
	c := qt.New(t)
	c.Assert(services.ThumbnailSizeNames(), qt.DeepEquals, []string{"small", "medium", "large"})
}

func TestIsThumbnailSize(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"small", true},
		{"medium", true},
		{"large", true},
		{"huge", false},
		{"", false},
		{"Large", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			c.Assert(services.IsThumbnailSize(tt.name), qt.Equals, tt.want)
		})
	}
}
