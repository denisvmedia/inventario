package services

import (
	"strings"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
)

// TestEmailTemplateRenderer_AllTypesAllLangsRender executes EVERY template
// type in EVERY language with a fully-populated job. Go templates type-check
// field access at Execute time (not Parse), so the startup parse in
// newEmailTemplateRenderer alone can't catch a bad `{{.Field}}` reference in
// a cs/ru translation — this does. #2090
func TestEmailTemplateRenderer_AllTypesAllLangsRender(t *testing.T) {
	c := qt.New(t)
	renderer, err := newEmailTemplateRenderer()
	c.Assert(err, qt.IsNil)

	changedAt := time.Date(2026, 2, 28, 7, 0, 0, 0, time.UTC)
	expiresAt := time.Date(2026, 3, 31, 7, 0, 0, 0, time.UTC)
	base := emailJob{
		To:                    "user@example.com",
		Name:                  "Alex",
		URL:                   "https://example.com/x",
		ChangedAt:             &changedAt,
		CommodityName:         "Drill",
		CommodityURL:          "https://example.com/c/1",
		ExpiryDate:            "2026-03-31",
		ThresholdDays:         30,
		InviterName:           "Sam",
		GroupName:             "Household",
		Role:                  "admin",
		ExpiresAt:             &expiresAt,
		ThresholdPercent:      90,
		UsagePercent:          92,
		StorageUsedHuman:      "135 MiB",
		StorageQuotaHuman:     "150 MiB",
		StorageBreakdownLines: []string{"Photos: 100 MiB"},
		StorageFilesURL:       "https://example.com/files",
		StorageSettingsURL:    "https://example.com/settings",
		BorrowerName:          "Pat",
		LentAt:                "2026-01-01",
		DueBackAt:             "2026-02-01",
		LoanKind:              "overdue",
		LoanDaysDelta:         5,
		MaintenanceTitle:      "Oil change",
		MaintenanceDueDate:    "2026-03-01",
		FeedbackType:          "Bug",
		FromName:              "Alex",
		FromEmail:             "alex@example.com",
		FeedbackMessage:       "hello",
		DiagnosticsLines:      []string{"x: y"},
	}
	types := []emailTemplateType{
		emailTemplateVerification, emailTemplatePasswordReset, emailTemplateMagicLink,
		emailTemplatePasswordChange, emailTemplateWelcome, emailTemplateWarrantyReminder,
		emailTemplateGroupInvite, emailTemplateStorageQuotaWarning, emailTemplateLoanReminder,
		emailTemplateMaintenanceReminder, emailTemplateFeedback,
	}
	for _, lang := range []string{"en", "cs", "ru"} {
		for _, tt := range types {
			t.Run(string(tt)+"/"+lang, func(t *testing.T) {
				c := qt.New(t)
				job := base
				job.TemplateType = tt
				job.Language = lang
				rendered, err := renderer.render(job)
				c.Assert(err, qt.IsNil)
				c.Assert(strings.TrimSpace(rendered.Subject), qt.Not(qt.Equals), "")
				c.Assert(strings.TrimSpace(rendered.HTML), qt.Not(qt.Equals), "")
				c.Assert(strings.TrimSpace(rendered.Text), qt.Not(qt.Equals), "")
			})
		}
	}
}

// TestEmailTemplateRenderer_SubjectLocalization pins the per-language subject
// resolution + the English fallback for unset/unknown languages. #2090
func TestEmailTemplateRenderer_SubjectLocalization(t *testing.T) {
	c := qt.New(t)
	renderer, err := newEmailTemplateRenderer()
	c.Assert(err, qt.IsNil)

	cases := []struct {
		name        string
		lang        string
		tt          emailTemplateType
		wantSubject string
	}{
		{"welcome en", "en", emailTemplateWelcome, "Welcome to Inventario"},
		{"welcome cs", "cs", emailTemplateWelcome, "Vítejte v Inventario"},
		{"welcome ru", "ru", emailTemplateWelcome, "Добро пожаловать в Inventario"},
		{"welcome empty -> en", "", emailTemplateWelcome, "Welcome to Inventario"},
		{"welcome unknown -> en", "xx", emailTemplateWelcome, "Welcome to Inventario"},
		{"verification ru", "ru", emailTemplateVerification, "Подтвердите свою учётную запись Inventario"},
		{"magic_link cs", "cs", emailTemplateMagicLink, "Přihlášení do Inventaria"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			rendered, err := renderer.render(emailJob{
				TemplateType: tc.tt,
				To:           "user@example.com",
				Name:         "Alex",
				URL:          "https://example.com/x",
				Language:     tc.lang,
			})
			c.Assert(err, qt.IsNil)
			c.Assert(rendered.Subject, qt.Equals, tc.wantSubject)
		})
	}
}

// TestEmailTemplateRenderer_BodyLocalized proves the localized template body
// is actually selected (cs/ru differ from en) and that an unset language
// falls back to the en body. #2090
func TestEmailTemplateRenderer_BodyLocalized(t *testing.T) {
	c := qt.New(t)
	renderer, err := newEmailTemplateRenderer()
	c.Assert(err, qt.IsNil)

	render := func(lang string) string {
		rendered, err := renderer.render(emailJob{
			TemplateType: emailTemplateWelcome,
			To:           "user@example.com",
			Name:         "Alex",
			Language:     lang,
		})
		c.Assert(err, qt.IsNil)
		return rendered.HTML
	}

	en := render("en")
	cs := render("cs")
	ru := render("ru")
	c.Assert(cs, qt.Not(qt.Equals), en, qt.Commentf("cs body should differ from en"))
	c.Assert(ru, qt.Not(qt.Equals), en, qt.Commentf("ru body should differ from en"))
	c.Assert(cs, qt.Contains, `lang="cs"`)
	c.Assert(ru, qt.Contains, `lang="ru"`)
	// Unset language falls back to the en body byte-for-byte.
	c.Assert(render(""), qt.Equals, en)
}

// TestEmailTemplateRenderer_FeedbackStaysEnglish — feedback is delivered to
// the operator inbox and must stay English even if the submitter's language
// is set. #2090
func TestEmailTemplateRenderer_FeedbackStaysEnglish(t *testing.T) {
	c := qt.New(t)
	renderer, err := newEmailTemplateRenderer()
	c.Assert(err, qt.IsNil)

	rendered, err := renderer.render(emailJob{
		TemplateType: emailTemplateFeedback,
		To:           "support@example.com",
		FeedbackType: "Bug",
		FromEmail:    "submitter@example.com",
		Language:     "cs",
	})
	c.Assert(err, qt.IsNil)
	c.Assert(rendered.Subject, qt.Equals, "[Inventario Bug] from submitter@example.com")
}

// TestEmailTemplateRenderer_LoanSubjectLocalized — the kind-aware loan subject
// is localized and the commodity name is concatenated (not fmt-interpolated),
// so a '%' in the name can't corrupt the subject. #2090
func TestEmailTemplateRenderer_LoanSubjectLocalized(t *testing.T) {
	c := qt.New(t)
	renderer, err := newEmailTemplateRenderer()
	c.Assert(err, qt.IsNil)

	ru, err := renderer.render(emailJob{
		TemplateType:  emailTemplateLoanReminder,
		To:            "user@example.com",
		CommodityName: "Drill",
		LoanKind:      "overdue",
		Language:      "ru",
	})
	c.Assert(err, qt.IsNil)
	c.Assert(ru.Subject, qt.Equals, "Напоминание: Drill просрочен")

	pct, err := renderer.render(emailJob{
		TemplateType:  emailTemplateLoanReminder,
		To:            "user@example.com",
		CommodityName: "100% cotton",
		LoanKind:      "due_soon",
		Language:      "en",
	})
	c.Assert(err, qt.IsNil)
	c.Assert(pct.Subject, qt.Equals, "100% cotton is due back soon")
}
