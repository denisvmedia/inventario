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
	subject, ok := subjectByTemplateType(tt)
	if !ok {
		return renderedEmail{}, fmt.Errorf("unsupported template type: %q", tt)
	}

	data := emailTemplateData{
		Name:             strings.TrimSpace(job.Name),
		URL:              job.URL,
		CommodityName:    strings.TrimSpace(job.CommodityName),
		CommodityURL:     strings.TrimSpace(job.CommodityURL),
		ExpiryDate:       strings.TrimSpace(job.ExpiryDate),
		ThresholdDays:    job.ThresholdDays,
		InviterName:      strings.TrimSpace(job.InviterName),
		GroupName:        strings.TrimSpace(job.GroupName),
		Role:             strings.TrimSpace(job.Role),
		ThresholdPercent: job.ThresholdPercent,
		UsagePercent:     job.UsagePercent,
		UsedHuman:        strings.TrimSpace(job.StorageUsedHuman),
		QuotaHuman:       strings.TrimSpace(job.StorageQuotaHuman),
		BreakdownLines:   job.StorageBreakdownLines,
		FilesURL:         strings.TrimSpace(job.StorageFilesURL),
		SettingsURL:      strings.TrimSpace(job.StorageSettingsURL),
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
	default:
		return "", false
	}
}
