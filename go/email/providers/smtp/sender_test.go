package smtp

import (
	"bufio"
	"context"
	"mime"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/email/sender"
)

type smtpTestServer struct {
	ln       net.Listener
	received chan string
}

func newSMTPTestServer(t *testing.T) *smtpTestServer {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen smtp test server: %v", err)
	}

	srv := &smtpTestServer{
		ln:       ln,
		received: make(chan string, 1),
	}
	go srv.serve()
	return srv
}

func (s *smtpTestServer) Addr() string {
	return s.ln.Addr().String()
}

func (s *smtpTestServer) Close() {
	_ = s.ln.Close()
}

func (s *smtpTestServer) serve() {
	conn, err := s.ln.Accept()
	if err != nil {
		return
	}
	defer func() { _ = conn.Close() }()

	r := bufio.NewReader(conn)
	w := bufio.NewWriter(conn)
	writeLine := func(line string) {
		_, _ = w.WriteString(line + "\r\n")
		_ = w.Flush()
	}

	writeLine("220 localhost ESMTP")

	inData := false
	var data strings.Builder
	for {
		line, readErr := r.ReadString('\n')
		if readErr != nil {
			return
		}
		line = strings.TrimRight(line, "\r\n")

		if inData {
			if line == "." {
				inData = false
				s.received <- data.String()
				data.Reset()
				writeLine("250 Ok: queued")
				continue
			}
			data.WriteString(line)
			data.WriteString("\r\n")
			continue
		}

		switch {
		case strings.HasPrefix(line, "EHLO"), strings.HasPrefix(line, "HELO"):
			_, _ = w.WriteString("250-localhost\r\n250 Ok\r\n")
			_ = w.Flush()
		case strings.HasPrefix(line, "MAIL FROM:"):
			writeLine("250 Ok")
		case strings.HasPrefix(line, "RCPT TO:"):
			writeLine("250 Ok")
		case line == "DATA":
			writeLine("354 End data with <CR><LF>.<CR><LF>")
			inData = true
		case line == "QUIT":
			writeLine("221 Bye")
			return
		default:
			writeLine("250 Ok")
		}
	}
}

func TestNew_RequiresHost(t *testing.T) {
	c := qt.New(t)

	_, err := New(Config{})
	c.Assert(err, qt.IsNotNil)
}

func TestNew_DefaultPort(t *testing.T) {
	c := qt.New(t)

	s, err := New(Config{Host: "smtp.example.com"})
	c.Assert(err, qt.IsNil)
	c.Assert(s.port, qt.Equals, 587)
}

func TestSender_Send(t *testing.T) {
	c := qt.New(t)

	srv := newSMTPTestServer(t)
	defer srv.Close()

	host, portRaw, err := net.SplitHostPort(srv.Addr())
	c.Assert(err, qt.IsNil)
	port, err := strconv.Atoi(portRaw)
	c.Assert(err, qt.IsNil)

	s, err := New(Config{
		Host: host,
		Port: port,
	})
	c.Assert(err, qt.IsNil)

	err = s.Send(context.Background(), sender.Message{
		To:      "user@example.com",
		From:    "noreply@example.com",
		ReplyTo: "support@example.com",
		Subject: "hello",
		HTML:    "<p>hello</p>",
		Text:    "hello",
	})
	c.Assert(err, qt.IsNil)

	select {
	case raw := <-srv.received:
		c.Assert(raw, qt.Contains, "Subject: hello")
		c.Assert(raw, qt.Contains, "Reply-To: support@example.com")
		c.Assert(raw, qt.Contains, "Content-Type: text/plain; charset=UTF-8")
		c.Assert(raw, qt.Contains, "Content-Type: text/html; charset=UTF-8")
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for SMTP DATA payload")
	}
}

// #2139: non-ASCII (cs/ru) subjects must be RFC 2047 encoded-words so mail
// clients render them correctly instead of garbling raw UTF-8.
func TestBuildMIMEMessage_EncodesNonASCIISubject(t *testing.T) {
	c := qt.New(t)
	raw := string(buildMIMEMessage(sender.Message{
		From:    "Inventario <noreply@example.com>",
		To:      "user@example.com",
		Subject: "Подтвердите свою учётную запись Inventario",
		Text:    "text",
		HTML:    "<p>html</p>",
	}))
	subject := mimeHeaderValue(c, raw, "Subject")
	// Raw Cyrillic must NOT appear unencoded in the header...
	c.Assert(subject, qt.Not(qt.Contains), "Подтвердите")
	c.Assert(strings.HasPrefix(subject, "=?UTF-8?"), qt.IsTrue)
	// ...and the encoded-word decodes back to the original.
	decoded, err := new(mime.WordDecoder).DecodeHeader(subject)
	c.Assert(err, qt.IsNil)
	c.Assert(decoded, qt.Equals, "Подтвердите свою учётную запись Inventario")
}

// #2139: a CRLF embedded in a header value (e.g. a commodity name in the
// Subject) must not be able to inject additional headers.
func TestBuildMIMEMessage_StripsHeaderInjection(t *testing.T) {
	c := qt.New(t)
	raw := string(buildMIMEMessage(sender.Message{
		From:    "noreply@example.com",
		To:      "user@example.com",
		Subject: "Hello\r\nBcc: attacker@example.com",
		Text:    "text",
		HTML:    "<p>html</p>",
	}))
	// No injected Bcc header line is created.
	c.Assert(raw, qt.Not(qt.Contains), "\r\nBcc:")
	// Everything collapses onto the single Subject line.
	c.Assert(mimeHeaderValue(c, raw, "Subject"), qt.Equals, "HelloBcc: attacker@example.com")
}

// #2139: encodeSubject leaves pure-ASCII subjects untouched and, for
// non-ASCII, emits the shorter of Q- and B-encoding (base64 wins for heavy
// Cyrillic — roughly halving the Russian subject, leaving more headroom
// under the RFC 5322 line limit, see #2142). The result must round-trip.
func TestEncodeSubject_PicksCompactEncoding(t *testing.T) {
	c := qt.New(t)
	// Pure ASCII passes through untouched (en subjects stay readable).
	c.Assert(encodeSubject("Welcome to Inventario"), qt.Equals, "Welcome to Inventario")

	for _, s := range []string{
		"Ověřte svůj účet Inventario",                // Czech
		"Подтвердите свою учётную запись Inventario", // Russian
	} {
		got := encodeSubject(s)
		c.Assert(strings.HasPrefix(got, "=?UTF-8?"), qt.IsTrue, qt.Commentf("got %q", got))
		decoded, err := new(mime.WordDecoder).DecodeHeader(got)
		c.Assert(err, qt.IsNil)
		c.Assert(decoded, qt.Equals, s)
		// The shorter of Q-/B-encoding was chosen.
		c.Assert(len(got) <= len(mime.QEncoding.Encode("UTF-8", s)), qt.IsTrue)
		c.Assert(len(got) <= len(mime.BEncoding.Encode("UTF-8", s)), qt.IsTrue)
	}
}

// mimeHeaderValue returns the value of the first `key: ` header in the
// header block (everything before the first blank line) of a raw message.
func mimeHeaderValue(c *qt.C, raw, key string) string {
	headers, _, ok := strings.Cut(raw, "\r\n\r\n")
	c.Assert(ok, qt.IsTrue)
	for line := range strings.SplitSeq(headers, "\r\n") {
		if v, found := strings.CutPrefix(line, key+": "); found {
			return v
		}
	}
	return ""
}
