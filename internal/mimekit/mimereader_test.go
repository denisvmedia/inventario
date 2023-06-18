package mimekit_test

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/internal/mimekit"
)

func TestMIMEReader_Read(t *testing.T) {
	t.Run("Expected content type", func(t *testing.T) {
		c := qt.New(t)

		contentTypes := []string{"text/plain", "application/json"}

		r := bytes.NewReader([]byte("Hello, World!"))
		mr := mimekit.NewMIMEReader(r, contentTypes)

		buf := make([]byte, 10)
		n, err := mr.Read(buf)
		c.Assert(err, qt.IsNil)
		c.Assert(n, qt.Equals, 10)

		// Make sure the bytes read match the input
		c.Assert(buf[:n], qt.DeepEquals, []byte("Hello, Wor"))

		// the content should not be detected yet (we have not read enough bytes, and we have not reached the end of the stream)
		n, err = mr.Read(buf)
		c.Assert(err, qt.IsNil)
		c.Assert(n, qt.Equals, 3)

		// Make sure the bytes read match the input
		c.Assert(buf[:n], qt.DeepEquals, []byte("ld!"))

		// Now it shouldn't read anything else, since the content type has been detected and the stream has ended.
		n, err = mr.Read(buf)
		c.Assert(err, qt.Equals, io.EOF)
		c.Assert(n, qt.Equals, 0)

		// Sequential reads should now return EOF
		n, err = mr.Read(buf)
		c.Assert(err, qt.Equals, io.EOF)
		c.Assert(n, qt.Equals, 0)
	})

	t.Run("Unexpected content type", func(t *testing.T) {
		c := qt.New(t)

		contentTypes := []string{"application/json"}

		r := bytes.NewReader([]byte("Hello, World!"))
		mr := mimekit.NewMIMEReader(r, contentTypes)

		buf := make([]byte, 10)
		n, err := mr.Read(buf)
		c.Assert(err, qt.IsNil)
		c.Assert(n, qt.Equals, 10)

		// Make sure the bytes read match the input
		c.Assert(buf[:n], qt.DeepEquals, []byte("Hello, Wor"))

		// the content should not be detected yet (we have not read enough bytes and we have not reached the end of the stream)
		n, err = mr.Read(buf)
		c.Assert(err, qt.IsNil)
		c.Assert(n, qt.Equals, 3)

		// Make sure the bytes read match the input
		c.Assert(buf[:n], qt.DeepEquals, []byte("ld!"))

		// Now it shouldn't read anything else, since the content type has been detected and the stream has ended.
		n, err = mr.Read(buf)
		c.Assert(err, qt.ErrorIs, mimekit.ErrInvalidContentType)
		c.Assert(n, qt.Equals, 0)

		// Sequential reads should now return the same error
		n, err = mr.Read(buf)
		c.Assert(err, qt.ErrorIs, mimekit.ErrInvalidContentType)
		c.Assert(n, qt.Equals, 0)
	})

	t.Run("Expected content type with more bytes", func(t *testing.T) {
		contentTypes := []string{"text/plain"}

		c := qt.New(t)

		data := []byte(strings.Repeat("Hello, World!", 1000)) // 13000 bytes

		r := bytes.NewReader(data)
		mr := mimekit.NewMIMEReader(r, contentTypes)

		read, err := io.ReadAll(mr)
		c.Assert(err, qt.IsNil)
		c.Assert(read, qt.DeepEquals, data)

		buf := make([]byte, 256)
		n, err := mr.Read(buf)
		c.Assert(err, qt.ErrorIs, io.EOF)
		c.Assert(n, qt.Equals, 0)
	})

	t.Run("Unexpected content type with more bytes", func(t *testing.T) {
		contentTypes := []string{"application/json"}

		c := qt.New(t)

		data := []byte(strings.Repeat("Hello, World!", 1000)) // 13000 bytes

		r := bytes.NewReader(data)
		mr := mimekit.NewMIMEReader(r, contentTypes)

		buf := make([]byte, 256) // half of sniffLen
		n, err := mr.Read(buf)
		c.Assert(err, qt.IsNil)

		// Make sure the bytes read match the input
		c.Assert(buf[:n], qt.DeepEquals, data[:n])

		// we have read enough bytes and the content should be invalid
		n, err = mr.Read(buf)
		c.Assert(err, qt.ErrorIs, mimekit.ErrInvalidContentType)
		c.Assert(n, qt.Equals, 256)

		// sequential calls should return the same error
		n, err = mr.Read(buf)
		c.Assert(err, qt.ErrorIs, mimekit.ErrInvalidContentType)
		c.Assert(n, qt.Equals, 0)
	})

	t.Run("Unexpected content type with more bytes with io.ReadAll", func(t *testing.T) {
		contentTypes := []string{"application/json"}

		c := qt.New(t)

		data := []byte(strings.Repeat("Hello, World!", 1000)) // 13000 bytes

		r := bytes.NewReader(data)
		mr := mimekit.NewMIMEReader(r, contentTypes)

		_, err := io.ReadAll(mr)
		c.Assert(err, qt.ErrorIs, mimekit.ErrInvalidContentType)
	})

	t.Run("Reader returned an error", func(t *testing.T) {
		contentTypes := []string{"application/json"}

		c := qt.New(t)

		expectedError := errors.New("some error")
		r := &errorReader{err: expectedError}
		mr := mimekit.NewMIMEReader(r, contentTypes)

		read, err := io.ReadAll(mr)
		c.Assert(err, qt.ErrorIs, expectedError)
		c.Assert(read, qt.DeepEquals, []byte("Hello, Wor"))

		read, err = io.ReadAll(mr)
		c.Assert(err, qt.ErrorIs, expectedError)
		c.Assert(read, qt.DeepEquals, []byte(""))
	})
}

type errorReader struct {
	err error
}

func (e *errorReader) Read(p []byte) (n int, err error) {
	copy(p[0:10], "Hello, Wor")
	return 10, e.err
}
