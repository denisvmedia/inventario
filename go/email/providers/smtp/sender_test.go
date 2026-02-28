package smtp

import (
	"bufio"
	"context"
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
