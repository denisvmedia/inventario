package services

// White-box: computeSubject, loanSubjects and subjectByTemplateType are
// unexported and there is no public entry point that returns a subject
// without also building the async service and its queue, so the language
// fallbacks and the loan-kind branches cannot be asserted through the
// exported API alone.

import (
	"testing"

	qt "github.com/frankban/quicktest"
)

func TestComputeSubject_LoanReminderByKindAndLanguage(t *testing.T) {
	c := qt.New(t)

	for _, tc := range []struct {
		name string
		lang string
		kind string
		item string
		want string
	}{
		{"en overdue", "en", "overdue", "Drill", "Reminder: Drill is overdue"},
		{"en due soon", "en", "due_soon", "Drill", "Drill is due back soon"},
		{"cs overdue", "cs", "overdue", "Vrtačka", "Připomenutí: Vrtačka je po termínu vrácení"},
		// Russian puts the qualifier before the noun, so the copy uses a
		// prefix where English uses a suffix. Asserting both directions is
		// the point — a set that only ever appends reads fine in English and
		// wrong everywhere else.
		{"ru due soon", "ru", "due_soon", "Дрель", "Скоро нужно вернуть Дрель"},
		// No name: the per-language placeholder stands in rather than
		// leaving a dangling "is overdue".
		{"en no name", "en", "overdue", "  ", "Reminder: your item is overdue"},
		{"ru no name", "ru", "due_soon", "", "Скоро нужно вернуть ваш предмет"},
	} {
		c.Run(tc.name, func(c *qt.C) {
			got, ok := computeSubject(emailJob{
				TemplateType:  emailTemplateLoanReminder,
				CommodityName: tc.item,
				LoanKind:      tc.kind,
			}, tc.lang)
			c.Assert(ok, qt.IsTrue)
			c.Assert(got, qt.Equals, tc.want)
		})
	}
}

// An unrecognised kind still produces a subject. Failing the render would
// drop a reminder the body could have delivered perfectly well.
func TestComputeSubject_UnknownLoanKindFallsBack(t *testing.T) {
	c := qt.New(t)

	got, ok := computeSubject(emailJob{
		TemplateType:  emailTemplateLoanReminder,
		CommodityName: "Drill",
		LoanKind:      "something_new",
	}, "en")
	c.Assert(ok, qt.IsTrue)
	c.Assert(got, qt.Equals, "Inventario loan reminder")
}

// An unknown language falls back to English rather than to an empty subject.
func TestLoanSubjects_UnknownLanguageFallsBackToEnglish(t *testing.T) {
	c := qt.New(t)

	// Compared through a rendered subject rather than the struct: the set is
	// unexported, and what callers actually depend on is the string.
	render := func(lang string) string {
		got, _ := computeSubject(emailJob{
			TemplateType:  emailTemplateLoanReminder,
			CommodityName: "Drill",
			LoanKind:      "overdue",
		}, lang)
		return got
	}
	c.Assert(render("de"), qt.Equals, render("en"))
	c.Assert(render(""), qt.Equals, render("en"))
}

// Feedback is operator-facing: it stays English whatever the recipient's
// language is, and carries the sender so the inbox can be triaged.
func TestComputeSubject_FeedbackIsAlwaysEnglish(t *testing.T) {
	c := qt.New(t)

	for _, lang := range []string{"en", "cs", "ru", "de"} {
		got, ok := computeSubject(emailJob{
			TemplateType: emailTemplateFeedback,
			FeedbackType: "Bug",
			FromEmail:    "user@example.com",
		}, lang)
		c.Assert(ok, qt.IsTrue)
		c.Assert(got, qt.Equals, "[Inventario Bug] from user@example.com")
	}

	// Blanks get stand-ins rather than producing "[Inventario ] from ".
	got, ok := computeSubject(emailJob{TemplateType: emailTemplateFeedback}, "en")
	c.Assert(ok, qt.IsTrue)
	c.Assert(got, qt.Equals, "[Inventario Feedback] from an Inventario user")
}

func TestSubjectByTemplateType_LanguageFallback(t *testing.T) {
	c := qt.New(t)

	en, ok := subjectByTemplateType(emailTemplateWelcome, "en")
	c.Assert(ok, qt.IsTrue)
	c.Assert(en, qt.Not(qt.Equals), "")

	// A translated language returns its own copy...
	cs, ok := subjectByTemplateType(emailTemplateWelcome, "cs")
	c.Assert(ok, qt.IsTrue)
	c.Assert(cs, qt.Not(qt.Equals), "")

	// ...and an unknown one falls back to English rather than to "".
	de, ok := subjectByTemplateType(emailTemplateWelcome, "de")
	c.Assert(ok, qt.IsTrue)
	c.Assert(de, qt.Equals, en)

	// An unknown template type has no subject at all, and says so — the
	// caller must not send a message with an empty Subject header.
	_, ok = subjectByTemplateType(emailTemplateType("no_such_template"), "en")
	c.Assert(ok, qt.IsFalse)
}
