package services

import (
	"testing"

	qt "github.com/frankban/quicktest"
)

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
