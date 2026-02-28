package services

import (
	"strings"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
)

func TestEmailTemplateRenderer_Render(t *testing.T) {
	c := qt.New(t)

	renderer, err := newEmailTemplateRenderer()
	c.Assert(err, qt.IsNil)

	changedAt := time.Date(2026, 2, 28, 7, 0, 0, 0, time.UTC)
	tests := []struct {
		name      string
		job       emailJob
		wantInAny []string
	}{
		{
			name: "verification template",
			job: emailJob{
				TemplateType: emailTemplateVerification,
				To:           "user@example.com",
				Name:         "Alex",
				URL:          "https://example.com/verify?token=abc",
			},
			wantInAny: []string{"Verify your Inventario account", "https://example.com/verify?token=abc", "Alex"},
		},
		{
			name: "password reset template",
			job: emailJob{
				TemplateType: emailTemplatePasswordReset,
				To:           "user@example.com",
				Name:         "Alex",
				URL:          "https://example.com/reset?token=abc",
			},
			wantInAny: []string{"Reset your Inventario password", "https://example.com/reset?token=abc", "Alex"},
		},
		{
			name: "password changed template",
			job: emailJob{
				TemplateType: emailTemplatePasswordChange,
				To:           "user@example.com",
				Name:         "Alex",
				ChangedAt:    &changedAt,
			},
			wantInAny: []string{"Your Inventario password was changed", "Alex", "2026"},
		},
		{
			name: "welcome template",
			job: emailJob{
				TemplateType: emailTemplateWelcome,
				To:           "user@example.com",
				Name:         "Alex",
			},
			wantInAny: []string{"Welcome to Inventario", "Alex", "Your account is now active"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			rendered, err := renderer.render(tt.job)
			c.Assert(err, qt.IsNil)
			c.Assert(strings.TrimSpace(rendered.Subject), qt.Not(qt.Equals), "")
			c.Assert(strings.TrimSpace(rendered.HTML), qt.Not(qt.Equals), "")
			c.Assert(strings.TrimSpace(rendered.Text), qt.Not(qt.Equals), "")

			full := rendered.Subject + "\n" + rendered.HTML + "\n" + rendered.Text
			for _, needle := range tt.wantInAny {
				c.Assert(full, qt.Contains, needle, qt.Commentf("expected %q in rendered output", needle))
			}
		})
	}
}
