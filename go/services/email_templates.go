package services

import (
	"bytes"
	"embed"
	"fmt"
	htemplate "html/template"
	"strings"
	ttemplate "text/template"
	"time"
)

// emailTemplatesFS embeds all transactional templates so deployment does not
// depend on runtime filesystem paths.
//
//go:embed email_templates/*.html.tmpl email_templates/*.txt.tmpl
var emailTemplatesFS embed.FS

type emailTemplateType string

const (
	emailTemplateVerification        emailTemplateType = "verification"
	emailTemplatePasswordReset       emailTemplateType = "password_reset"
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
type emailTemplateRenderer struct {
	htmlTemplates map[emailTemplateType]*htemplate.Template
	textTemplates map[emailTemplateType]*ttemplate.Template
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

// newEmailTemplateRenderer parses all embedded template files and builds a
// renderer instance used by AsyncEmailService workers.
func newEmailTemplateRenderer() (*emailTemplateRenderer, error) {
	renderer := &emailTemplateRenderer{
		htmlTemplates: make(map[emailTemplateType]*htemplate.Template),
		textTemplates: make(map[emailTemplateType]*ttemplate.Template),
	}
	// #nosec G101 -- these are template file paths, not credentials.
	htmlTemplateFiles := map[emailTemplateType]string{
		emailTemplateVerification:        "email_templates/verification.html.tmpl",
		emailTemplatePasswordReset:       "email_templates/password_reset.html.tmpl",
		emailTemplatePasswordChange:      "email_templates/password_changed.html.tmpl",
		emailTemplateWelcome:             "email_templates/welcome.html.tmpl",
		emailTemplateWarrantyReminder:    "email_templates/warranty_reminder.html.tmpl",
		emailTemplateGroupInvite:         "email_templates/group_invite.html.tmpl",
		emailTemplateStorageQuotaWarning: "email_templates/storage_quota_warning.html.tmpl",
		emailTemplateLoanReminder:        "email_templates/loan_reminder.html.tmpl",
		emailTemplateMaintenanceReminder: "email_templates/maintenance_reminder.html.tmpl",
		emailTemplateFeedback:            "email_templates/feedback.html.tmpl",
	}
	// #nosec G101 -- these are template file paths, not credentials.
	textTemplateFiles := map[emailTemplateType]string{
		emailTemplateVerification:        "email_templates/verification.txt.tmpl",
		emailTemplatePasswordReset:       "email_templates/password_reset.txt.tmpl",
		emailTemplatePasswordChange:      "email_templates/password_changed.txt.tmpl",
		emailTemplateWelcome:             "email_templates/welcome.txt.tmpl",
		emailTemplateWarrantyReminder:    "email_templates/warranty_reminder.txt.tmpl",
		emailTemplateGroupInvite:         "email_templates/group_invite.txt.tmpl",
		emailTemplateStorageQuotaWarning: "email_templates/storage_quota_warning.txt.tmpl",
		emailTemplateLoanReminder:        "email_templates/loan_reminder.txt.tmpl",
		emailTemplateMaintenanceReminder: "email_templates/maintenance_reminder.txt.tmpl",
		emailTemplateFeedback:            "email_templates/feedback.txt.tmpl",
	}

	for tt, file := range htmlTemplateFiles {
		raw, err := emailTemplatesFS.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("read html template %q: %w", file, err)
		}
		tmpl, err := htemplate.New(string(tt)).Parse(string(raw))
		if err != nil {
			return nil, fmt.Errorf("parse html template %q: %w", file, err)
		}
		renderer.htmlTemplates[tt] = tmpl
	}

	for tt, file := range textTemplateFiles {
		raw, err := emailTemplatesFS.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("read text template %q: %w", file, err)
		}
		tmpl, err := ttemplate.New(string(tt)).Parse(string(raw))
		if err != nil {
			return nil, fmt.Errorf("parse text template %q: %w", file, err)
		}
		renderer.textTemplates[tt] = tmpl
	}

	return renderer, nil
}

// render merges a logical emailJob with template data and returns the subject
// plus HTML/text bodies required by sender.Message.
func (r *emailTemplateRenderer) render(job emailJob) (renderedEmail, error) {
	tt := job.TemplateType
	subject, ok := computeSubject(job)
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

	htmlTmpl, ok := r.htmlTemplates[tt]
	if !ok {
		return renderedEmail{}, fmt.Errorf("missing html template: %q", tt)
	}
	textTmpl, ok := r.textTemplates[tt]
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

// computeSubject is the kind-aware subject builder. Most templates have
// a fixed subject (delegated to subjectByTemplateType); the loan
// reminder needs to interpolate the commodity name and branch on the
// LoanKind, so it gets its own switch.
func computeSubject(job emailJob) (string, bool) {
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
		name := strings.TrimSpace(job.CommodityName)
		if name == "" {
			name = "your item"
		}
		switch job.LoanKind {
		case "overdue":
			return "Reminder: " + name + " is overdue", true
		case "due_soon":
			return name + " is due back soon", true
		default:
			// Unknown kind — return a generic subject rather than failing
			// the render outright. The body still surfaces the loan
			// details so the recipient isn't left guessing.
			return "Inventario loan reminder", true
		}
	}
	return subjectByTemplateType(job.TemplateType)
}

func subjectByTemplateType(tt emailTemplateType) (string, bool) {
	switch tt {
	case emailTemplateVerification:
		return "Verify your Inventario account", true
	case emailTemplatePasswordReset:
		return "Reset your Inventario password", true
	case emailTemplatePasswordChange:
		return "Your Inventario password was changed", true
	case emailTemplateWelcome:
		return "Welcome to Inventario", true
	case emailTemplateWarrantyReminder:
		return "Inventario warranty reminder", true
	case emailTemplateGroupInvite:
		return "You're invited to a group on Inventario", true
	case emailTemplateStorageQuotaWarning:
		return "Your group is approaching its storage quota", true
	case emailTemplateLoanReminder:
		return "Inventario loan reminder", true
	case emailTemplateMaintenanceReminder:
		return "Inventario maintenance reminder", true
	case emailTemplateFeedback:
		// Subject for feedback is built dynamically by computeSubject so
		// it can surface the submitter's address; this branch only
		// exists so emailTemplateFeedback is recognised as a valid type
		// at enqueue time (see AsyncEmailService.enqueue).
		return "Inventario feedback", true
	default:
		return "", false
	}
}
