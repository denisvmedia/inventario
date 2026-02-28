package smtp

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	stdsmtp "net/smtp"
	"strconv"
	"strings"
	"time"

	"github.com/denisvmedia/inventario/email/sender"
)

// Config defines SMTP transport settings used by New.
//
// The sender uses these values to establish a connection, optionally upgrade it
// to TLS with STARTTLS, and authenticate before issuing SMTP commands.
type Config struct {
	// Host is the SMTP server host (required).
	Host string
	// Port is the SMTP server port; defaults to 587 when zero.
	Port int
	// Username is the optional SMTP username for AUTH.
	Username string
	// Password is the optional SMTP password for AUTH.
	Password string
	// UseTLS requires STARTTLS before credentials/body are sent.
	UseTLS bool
}

// Sender delivers email through an SMTP server using net/smtp.
//
// It assembles a multipart/alternative MIME payload so text-only and HTML-capable
// clients can consume the same message.
// Sender delivers email through an SMTP server.
type Sender struct {
	host     string
	port     int
	username string
	password string
	useTLS   bool
}

// New creates an SMTP sender from config and applies safe defaults.
func New(cfg Config) (*Sender, error) {
	if strings.TrimSpace(cfg.Host) == "" {
		return nil, fmt.Errorf("SMTP provider requires SMTP_HOST")
	}
	if cfg.Port <= 0 {
		cfg.Port = 587
	}
	return &Sender{
		host:     strings.TrimSpace(cfg.Host),
		port:     cfg.Port,
		username: cfg.Username,
		password: cfg.Password,
		useTLS:   cfg.UseTLS,
	}, nil
}

// Send performs one SMTP transaction:
// dial -> optional STARTTLS -> optional AUTH -> MAIL/RCPT/DATA -> QUIT.
//
// Any transport or protocol error is returned to the caller for centralized
// retry/backoff handling.
func (s *Sender) Send(ctx context.Context, message sender.Message) error {
	addr := net.JoinHostPort(s.host, strconv.Itoa(s.port))
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("smtp dial: %w", err)
	}

	if deadline, ok := ctx.Deadline(); ok {
		if err := conn.SetDeadline(deadline); err != nil {
			_ = conn.Close()
			return fmt.Errorf("smtp set deadline: %w", err)
		}
	}

	client, err := stdsmtp.NewClient(conn, s.host)
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("smtp new client: %w", err)
	}
	defer func() { _ = client.Close() }()

	if s.useTLS {
		if ok, _ := client.Extension("STARTTLS"); !ok {
			return fmt.Errorf("smtp server %s does not support STARTTLS", s.host)
		}
		tlsConfig := &tls.Config{
			ServerName: s.host,
			MinVersion: tls.VersionTLS12,
		}
		if err := client.StartTLS(tlsConfig); err != nil {
			return fmt.Errorf("smtp starttls: %w", err)
		}
	}

	if s.username != "" {
		auth := stdsmtp.PlainAuth("", s.username, s.password, s.host)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("smtp auth: %w", err)
		}
	}

	if err := client.Mail(message.From); err != nil {
		return fmt.Errorf("smtp MAIL FROM: %w", err)
	}
	if err := client.Rcpt(message.To); err != nil {
		return fmt.Errorf("smtp RCPT TO: %w", err)
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp DATA: %w", err)
	}

	_, writeErr := w.Write(buildMIMEMessage(message))
	closeErr := w.Close()
	if writeErr != nil {
		return fmt.Errorf("smtp write body: %w", writeErr)
	}
	if closeErr != nil {
		return fmt.Errorf("smtp close DATA writer: %w", closeErr)
	}

	if err := client.Quit(); err != nil {
		return fmt.Errorf("smtp quit: %w", err)
	}
	return nil
}

func buildMIMEMessage(message sender.Message) []byte {
	boundary := fmt.Sprintf("inventario-%d", time.Now().UnixNano())
	var b strings.Builder

	fmt.Fprintf(&b, "From: %s\r\n", message.From)
	fmt.Fprintf(&b, "To: %s\r\n", message.To)
	fmt.Fprintf(&b, "Subject: %s\r\n", message.Subject)
	if strings.TrimSpace(message.ReplyTo) != "" {
		fmt.Fprintf(&b, "Reply-To: %s\r\n", message.ReplyTo)
	}
	b.WriteString("MIME-Version: 1.0\r\n")
	fmt.Fprintf(&b, "Content-Type: multipart/alternative; boundary=%q\r\n", boundary)
	b.WriteString("\r\n")

	fmt.Fprintf(&b, "--%s\r\n", boundary)
	b.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	b.WriteString("Content-Transfer-Encoding: 8bit\r\n\r\n")
	b.WriteString(message.Text)
	b.WriteString("\r\n")

	fmt.Fprintf(&b, "--%s\r\n", boundary)
	b.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	b.WriteString("Content-Transfer-Encoding: 8bit\r\n\r\n")
	b.WriteString(message.HTML)
	b.WriteString("\r\n")

	fmt.Fprintf(&b, "--%s--\r\n", boundary)
	return []byte(b.String())
}
