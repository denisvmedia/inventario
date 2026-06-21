package services

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	htemplate "html/template"
	"io/fs"
	"strings"
	ttemplate "text/template"
	"time"
)

// emailTemplatesFS embeds all transactional templates so deployment does not
// depend on runtime filesystem paths.
//
//go:embed email_templates/*.html.tmpl email_templates/*.txt.tmpl email_templates/cs/*.html.tmpl email_templates/cs/*.txt.tmpl email_templates/ru/*.html.tmpl email_templates/ru/*.txt.tmpl
var emailTemplatesFS embed.FS

type emailTemplateType string

const (
	emailTemplateVerification        emailTemplateType = "verification"
	emailTemplatePasswordReset       emailTemplateType = "password_reset"
	emailTemplateMagicLink           emailTemplateType = "magic_link"
	emailTemplatePasswordChange      emailTemplateType = "password_changed"
	emailTemplateWelcome             emailTemplateType = "welcome"
	emailTemplateWarrantyReminder    emailTemplateType = "warranty_reminder"
	emailTemplateGroupInvite         emailTemplateType = "group_invite"
	emailTemplateStorageQuotaWarning emailTemplateType = "storage_quota_warning"
	emailTemplateLoanReminder        emailTemplateType = "loan_reminder"
	emailTemplateMaintenanceReminder emailTemplateType = "maintenance_reminder"
	emailTemplateFeedback            emailTemplateType = "feedback"
)

type renderedEmail struct {
	Subject string
	HTML    string
	Text    string
}

// emailTemplateRenderer caches parsed templates for each transactional type.
//
// Parsing happens once at startup; rendering executes templates per job.
// Keyed by language ("en"/"cs"/"ru") then template type. The "en" maps are
// always fully populated; cs/ru hold the localized subset (feedback is
// operator-facing and stays English). A missing (lang, type) entry falls
// back to en at render time. #2090
type emailTemplateRenderer struct {
	htmlTemplates map[string]map[emailTemplateType]*htemplate.Template
	textTemplates map[string]map[emailTemplateType]*ttemplate.Template
}

type emailTemplateData struct {
	Name      string
	URL       string
	ChangedAt string
	// Warranty-reminder fields. Empty for every other template type.
	CommodityName string
	CommodityURL  string
	ExpiryDate    string
	ThresholdDays int
	// Group-invite fields. Empty for every other template type.
	InviterName string
	GroupName   string
	Role        string
	ExpiresAt   string
	// Storage-quota fields. Empty for every other template type.
	// ThresholdPercent is the matched tier (e.g. 90); UsagePercent
	// is the actual rounded percentage at send time. UsedHuman /
	// QuotaHuman are pre-formatted (e.g. "135 MiB"). BreakdownLines
	// is the per-bucket label slice rendered into a bullet list.
	// FilesURL / SettingsURL may be empty: the template suppresses
	// the matching link block when so.
	ThresholdPercent int
	UsagePercent     int
	UsedHuman        string
	QuotaHuman       string
	BreakdownLines   []string
	FilesURL         string
	SettingsURL      string
	// Loan-reminder fields (#1509). LoanKind is "overdue" | "due_soon";
	// LoanDaysDelta is the positive magnitude (days-until-due for
	// due_soon, days-overdue for overdue). The template branches on
	// LoanKind to pick the right copy variant.
	BorrowerName  string
	LentAt        string
	DueBackAt     string
	LoanKind      string
	LoanDaysDelta int
	// LoanIsOverdue / LoanIsDueSoon are convenience flags derived from
	// LoanKind so the template can use {{if .LoanIsOverdue}} ... {{else
	// if .LoanIsDueSoon}} branches without re-implementing the string
	// equality check.
	LoanIsOverdue bool
	LoanIsDueSoon bool
	// Maintenance-reminder fields. Empty for every other template type.
	// CommodityName / CommodityURL / ThresholdDays are shared with the
	// warranty template; Title is the user-supplied schedule label and
	// DueDate is the next_due_at formatted as YYYY-MM-DD.
	MaintenanceTitle   string
	MaintenanceDueDate string
	// Feedback fields (#1387). Populated only by
	// AsyncEmailService.SendFeedbackEmail. FeedbackType is the human
	// label ("Bug", "Feature request", etc.) the renderer surfaces in
	// the subject and intro line. FromName/FromEmail/FromUserID identify
	// the submitter; the submitter's reply-to address (when provided)
	// goes into ReplyToEmail. DiagnosticsLines is a pre-formatted slice
	// of "label: value" strings rendered into a bullet list.
	FeedbackType     string
	FromName         string
	FromEmail        string
	FromUserID       string
	ReplyToEmail     string
	Message          string
	DiagnosticsLines []string
}

// emailTemplateLanguages lists the locales we ship templates + subjects
// for. "en" is the canonical, always-complete catalog; cs/ru are localized
// subsets that fall back to en per type at render time. #2090
var emailTemplateLanguages = []string{"en", "cs", "ru"}

// emailTemplateBasenames maps each template type to its file basename
// (shared across languages); the ".html.tmpl"/".txt.tmpl" suffix is
// appended by the loader.
var emailTemplateBasenames = map[emailTemplateType]string{
	emailTemplateVerification:        "verification",
	emailTemplatePasswordReset:       "password_reset",
	emailTemplateMagicLink:           "magic_link",
	emailTemplatePasswordChange:      "password_changed",
	emailTemplateWelcome:             "welcome",
	emailTemplateWarrantyReminder:    "warranty_reminder",
	emailTemplateGroupInvite:         "group_invite",
	emailTemplateStorageQuotaWarning: "storage_quota_warning",
	emailTemplateLoanReminder:        "loan_reminder",
	emailTemplateMaintenanceReminder: "maintenance_reminder",
	emailTemplateFeedback:            "feedback",
}

// emailTemplatePath resolves the embedded path for (lang, basename, suffix).
// en templates live at the root; cs/ru in per-language subdirs.
func emailTemplatePath(lang, base, suffix string) string {
	// #nosec G101 -- these are template file paths, not credentials.
	if lang == "en" {
		return "email_templates/" + base + suffix
	}
	return "email_templates/" + lang + "/" + base + suffix
}

// newEmailTemplateRenderer parses all embedded template files (en + cs + ru)
// and builds a renderer instance used by AsyncEmailService workers. A
// localized (cs/ru) variant that is absent is skipped — render() falls back
// to the en template for that type.
func newEmailTemplateRenderer() (*emailTemplateRenderer, error) {
	renderer := &emailTemplateRenderer{
		htmlTemplates: make(map[string]map[emailTemplateType]*htemplate.Template),
		textTemplates: make(map[string]map[emailTemplateType]*ttemplate.Template),
	}

	for _, lang := range emailTemplateLanguages {
		renderer.htmlTemplates[lang] = make(map[emailTemplateType]*htemplate.Template)
		renderer.textTemplates[lang] = make(map[emailTemplateType]*ttemplate.Template)

		for tt, base := range emailTemplateBasenames {
			htmlPath := emailTemplatePath(lang, base, ".html.tmpl")
			rawHTML, err := emailTemplatesFS.ReadFile(htmlPath)
			if err != nil {
				// A localized variant may legitimately be absent (e.g. cs/ru
				// have no feedback template); fall back to en at render time.
				if lang != "en" && errors.Is(err, fs.ErrNotExist) {
					continue
				}
				return nil, fmt.Errorf("read html template %q: %w", htmlPath, err)
			}
			htmlTmpl, err := htemplate.New(lang + ":" + string(tt)).Parse(string(rawHTML))
			if err != nil {
				return nil, fmt.Errorf("parse html template %q: %w", htmlPath, err)
			}
			renderer.htmlTemplates[lang][tt] = htmlTmpl

			textPath := emailTemplatePath(lang, base, ".txt.tmpl")
			rawText, err := emailTemplatesFS.ReadFile(textPath)
			if err != nil {
				return nil, fmt.Errorf("read text template %q: %w", textPath, err)
			}
			textTmpl, err := ttemplate.New(lang + ":" + string(tt)).Parse(string(rawText))
			if err != nil {
				return nil, fmt.Errorf("parse text template %q: %w", textPath, err)
			}
			renderer.textTemplates[lang][tt] = textTmpl
		}
	}

	return renderer, nil
}

// templateForHTML returns the html template for (lang, type), falling back
// to the en template when no localized variant exists.
func (r *emailTemplateRenderer) templateForHTML(lang string, tt emailTemplateType) (*htemplate.Template, bool) {
	if m, ok := r.htmlTemplates[lang]; ok {
		if t, ok := m[tt]; ok {
			return t, true
		}
	}
	t, ok := r.htmlTemplates["en"][tt]
	return t, ok
}

// templateForText mirrors templateForHTML for the plain-text variant.
func (r *emailTemplateRenderer) templateForText(lang string, tt emailTemplateType) (*ttemplate.Template, bool) {
	if m, ok := r.textTemplates[lang]; ok {
		if t, ok := m[tt]; ok {
			return t, true
		}
	}
	t, ok := r.textTemplates["en"][tt]
	return t, ok
}

// render merges a logical emailJob with template data and returns the subject
// plus HTML/text bodies required by sender.Message.
func (r *emailTemplateRenderer) render(job emailJob) (renderedEmail, error) {
	tt := job.TemplateType
	lang := normalizeEmailLang(job.Language)
	subject, ok := computeSubject(job, lang)
	if !ok {
		return renderedEmail{}, fmt.Errorf("unsupported template type: %q", tt)
	}

	data := emailTemplateData{
		Name:               strings.TrimSpace(job.Name),
		URL:                job.URL,
		CommodityName:      strings.TrimSpace(job.CommodityName),
		CommodityURL:       strings.TrimSpace(job.CommodityURL),
		ExpiryDate:         strings.TrimSpace(job.ExpiryDate),
		ThresholdDays:      job.ThresholdDays,
		InviterName:        strings.TrimSpace(job.InviterName),
		GroupName:          strings.TrimSpace(job.GroupName),
		Role:               strings.TrimSpace(job.Role),
		ThresholdPercent:   job.ThresholdPercent,
		UsagePercent:       job.UsagePercent,
		UsedHuman:          strings.TrimSpace(job.StorageUsedHuman),
		QuotaHuman:         strings.TrimSpace(job.StorageQuotaHuman),
		BreakdownLines:     job.StorageBreakdownLines,
		FilesURL:           strings.TrimSpace(job.StorageFilesURL),
		SettingsURL:        strings.TrimSpace(job.StorageSettingsURL),
		BorrowerName:       strings.TrimSpace(job.BorrowerName),
		LentAt:             strings.TrimSpace(job.LentAt),
		DueBackAt:          strings.TrimSpace(job.DueBackAt),
		LoanKind:           strings.TrimSpace(job.LoanKind),
		LoanDaysDelta:      job.LoanDaysDelta,
		LoanIsOverdue:      job.LoanKind == "overdue",
		LoanIsDueSoon:      job.LoanKind == "due_soon",
		MaintenanceTitle:   strings.TrimSpace(job.MaintenanceTitle),
		MaintenanceDueDate: strings.TrimSpace(job.MaintenanceDueDate),
		FeedbackType:       strings.TrimSpace(job.FeedbackType),
		FromName:           strings.TrimSpace(job.FromName),
		FromEmail:          strings.TrimSpace(job.FromEmail),
		FromUserID:         strings.TrimSpace(job.FromUserID),
		ReplyToEmail:       strings.TrimSpace(job.ReplyToEmail),
		Message:            job.FeedbackMessage,
		DiagnosticsLines:   job.DiagnosticsLines,
	}
	if data.Name == "" {
		data.Name = "there"
	}
	if data.InviterName == "" {
		data.InviterName = "An Inventario user"
	}
	if job.ChangedAt != nil {
		data.ChangedAt = job.ChangedAt.UTC().Format(time.RFC1123)
	}
	if job.ExpiresAt != nil {
		data.ExpiresAt = job.ExpiresAt.UTC().Format(time.RFC1123)
	}

	htmlTmpl, ok := r.templateForHTML(lang, tt)
	if !ok {
		return renderedEmail{}, fmt.Errorf("missing html template: %q", tt)
	}
	textTmpl, ok := r.templateForText(lang, tt)
	if !ok {
		return renderedEmail{}, fmt.Errorf("missing text template: %q", tt)
	}

	var htmlBuf bytes.Buffer
	if err := htmlTmpl.Execute(&htmlBuf, data); err != nil {
		return renderedEmail{}, fmt.Errorf("render html template %q: %w", tt, err)
	}

	var textBuf bytes.Buffer
	if err := textTmpl.Execute(&textBuf, data); err != nil {
		return renderedEmail{}, fmt.Errorf("render text template %q: %w", tt, err)
	}

	return renderedEmail{
		Subject: subject,
		HTML:    htmlBuf.String(),
		Text:    textBuf.String(),
	}, nil
}

// normalizeEmailLang collapses an arbitrary language code to one of the
// locales we ship (en/cs/ru); anything unknown (including "") → "en".
func normalizeEmailLang(lang string) string {
	switch lang {
	case "cs", "ru":
		return lang
	default:
		return "en"
	}
}

// emailSubjects holds the fixed per-type subject line for each language.
// cs/ru omit feedback (operator-facing, English only); subjectByTemplateType
// falls back to en for any missing (lang, type). #2090
var emailSubjects = map[string]map[emailTemplateType]string{
	"en": { // #nosec G101 -- email subject lines, not credentials
		emailTemplateVerification:        "Verify your Inventario account",
		emailTemplatePasswordReset:       "Reset your Inventario password",
		emailTemplateMagicLink:           "Sign in to Inventario",
		emailTemplatePasswordChange:      "Your Inventario password was changed",
		emailTemplateWelcome:             "Welcome to Inventario",
		emailTemplateWarrantyReminder:    "Inventario warranty reminder",
		emailTemplateGroupInvite:         "You're invited to a group on Inventario",
		emailTemplateStorageQuotaWarning: "Your group is approaching its storage quota",
		emailTemplateLoanReminder:        "Inventario loan reminder",
		emailTemplateMaintenanceReminder: "Inventario maintenance reminder",
		emailTemplateFeedback:            "Inventario feedback",
	},
	"cs": { // #nosec G101 -- email subject lines, not credentials
		emailTemplateVerification:        "Ověřte svůj účet Inventario",
		emailTemplatePasswordReset:       "Obnovení hesla pro Inventario",
		emailTemplateMagicLink:           "Přihlášení do Inventaria",
		emailTemplatePasswordChange:      "Vaše heslo k Inventario bylo změněno",
		emailTemplateWelcome:             "Vítejte v Inventario",
		emailTemplateWarrantyReminder:    "Připomenutí záruky Inventario",
		emailTemplateGroupInvite:         "Máte pozvánku do skupiny v Inventariu",
		emailTemplateStorageQuotaWarning: "Vaše skupina se blíží svému úložnému limitu",
		emailTemplateLoanReminder:        "Připomenutí zápůjčky Inventario",
		emailTemplateMaintenanceReminder: "Připomenutí údržby v Inventariu",
	},
	"ru": { // #nosec G101 -- email subject lines, not credentials
		emailTemplateVerification:        "Подтвердите свою учётную запись Inventario",
		emailTemplatePasswordReset:       "Сброс пароля в Inventario",
		emailTemplateMagicLink:           "Вход в Inventario",
		emailTemplatePasswordChange:      "Ваш пароль Inventario был изменён",
		emailTemplateWelcome:             "Добро пожаловать в Inventario",
		emailTemplateWarrantyReminder:    "Напоминание о гарантии Inventario",
		emailTemplateGroupInvite:         "Вас пригласили в группу в Inventario",
		emailTemplateStorageQuotaWarning: "Ваша группа приближается к лимиту квоты хранилища",
		emailTemplateLoanReminder:        "Напоминание о займе Inventario",
		emailTemplateMaintenanceReminder: "Напоминание об обслуживании в Inventario",
	},
}

// loanSubjectSet holds the kind-aware loan-reminder subject parts. The
// commodity name is concatenated (never fmt-interpolated) so a name
// containing '%' can't corrupt the subject.
type loanSubjectSet struct {
	overduePrefix string
	overdueSuffix string
	dueSoonPrefix string
	dueSoonSuffix string
	def           string
	itemFallback  string
}

var loanSubjectsByLang = map[string]loanSubjectSet{
	"en": {"Reminder: ", " is overdue", "", " is due back soon", "Inventario loan reminder", "your item"},
	"cs": {"Připomenutí: ", " je po termínu vrácení", "", " se brzy blíží termín vrácení", "Připomenutí zápůjčky Inventario", "vaši položku"},
	"ru": {"Напоминание: ", " просрочен", "Скоро нужно вернуть ", "", "Напоминание о займе Inventario", "ваш предмет"},
}

func loanSubjects(lang string) loanSubjectSet {
	if s, ok := loanSubjectsByLang[lang]; ok {
		return s
	}
	return loanSubjectsByLang["en"]
}

// computeSubject is the kind-aware subject builder. Most templates have a
// fixed per-language subject (subjectByTemplateType); the loan reminder
// interpolates the commodity name and branches on LoanKind; feedback is
// operator-facing and stays English regardless of the recipient language.
func computeSubject(job emailJob, lang string) (string, bool) {
	if job.TemplateType == emailTemplateFeedback {
		feedbackType := strings.TrimSpace(job.FeedbackType)
		if feedbackType == "" {
			feedbackType = "Feedback"
		}
		from := strings.TrimSpace(job.FromEmail)
		if from == "" {
			from = "an Inventario user"
		}
		return fmt.Sprintf("[Inventario %s] from %s", feedbackType, from), true
	}
	if job.TemplateType == emailTemplateLoanReminder {
		ls := loanSubjects(lang)
		name := strings.TrimSpace(job.CommodityName)
		if name == "" {
			name = ls.itemFallback
		}
		switch job.LoanKind {
		case "overdue":
			return ls.overduePrefix + name + ls.overdueSuffix, true
		case "due_soon":
			return ls.dueSoonPrefix + name + ls.dueSoonSuffix, true
		default:
			// Unknown kind — return a generic subject rather than failing
			// the render outright. The body still surfaces the loan
			// details so the recipient isn't left guessing.
			return ls.def, true
		}
	}
	return subjectByTemplateType(job.TemplateType, lang)
}

// subjectByTemplateType returns the fixed subject for a template type in the
// requested language, falling back to en when the (lang, type) is absent.
func subjectByTemplateType(tt emailTemplateType, lang string) (string, bool) {
	if m, ok := emailSubjects[lang]; ok {
		if s, ok := m[tt]; ok {
			return s, true
		}
	}
	if s, ok := emailSubjects["en"][tt]; ok {
		return s, true
	}
	return "", false
}
