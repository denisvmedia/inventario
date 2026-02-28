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
	emailTemplateVerification   emailTemplateType = "verification"
	emailTemplatePasswordReset  emailTemplateType = "password_reset"
	emailTemplatePasswordChange emailTemplateType = "password_changed"
	emailTemplateWelcome        emailTemplateType = "welcome"
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
		emailTemplateVerification:   "email_templates/verification.html.tmpl",
		emailTemplatePasswordReset:  "email_templates/password_reset.html.tmpl",
		emailTemplatePasswordChange: "email_templates/password_changed.html.tmpl",
		emailTemplateWelcome:        "email_templates/welcome.html.tmpl",
	}
	// #nosec G101 -- these are template file paths, not credentials.
	textTemplateFiles := map[emailTemplateType]string{
		emailTemplateVerification:   "email_templates/verification.txt.tmpl",
		emailTemplatePasswordReset:  "email_templates/password_reset.txt.tmpl",
		emailTemplatePasswordChange: "email_templates/password_changed.txt.tmpl",
		emailTemplateWelcome:        "email_templates/welcome.txt.tmpl",
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
		Name: strings.TrimSpace(job.Name),
		URL:  job.URL,
	}
	if data.Name == "" {
		data.Name = "there"
	}
	if job.ChangedAt != nil {
		data.ChangedAt = job.ChangedAt.UTC().Format(time.RFC1123)
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
	default:
		return "", false
	}
}
