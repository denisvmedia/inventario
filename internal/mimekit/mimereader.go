package mimekit

import (
	"bytes"
	"io"
	"log"
	"mime"

	"github.com/gabriel-vasile/mimetype"
	"golang.org/x/exp/slices"

	"github.com/denisvmedia/inventario/internal/errkit"
)

// The sniff algorithm uses at most sniffLen bytes to make its decision.
const sniffLen = 512

// MIMEReader represents a reader that performs MIME type detection and validation on data read from an underlying io.Reader.
type MIMEReader struct {
	r                   io.Reader
	read                int64
	buf                 bytes.Buffer
	allowedContentTypes []string
	mimetype            string
	err                 error
}

// NewMIMEReader creates a new MIMEReader instance with the provided io.Reader and a list of allowed content types.
//
// The MIMEReader is designed to read data from the underlying reader while performing MIME type detection and validation.
// It allows developers to ensure that the content being read conforms to the expected MIME types.
// The provided allowedContentTypes is a list of MIME types that are considered valid.
//
// It will stop reading from the underlying reader as soon it detects disallowed content type.
func NewMIMEReader(r io.Reader, allowedContentTypes []string) *MIMEReader {
	return &MIMEReader{
		r:                   r,
		allowedContentTypes: allowedContentTypes,
	}
}

// Read reads data from the underlying reader into the provided byte slice,
// performs MIME type detection and validation, and returns the number of bytes read and an error (if any).
//
//   - If the MIME type has already been detected, the method reads data until the end of the underlying reader.
//   - If the MIME type has not been detected, the method continues reading and buffering data until the detection criteria are met.
//   - Once the detection criteria are met, the buffered data is used to detect the MIME type using http.DetectContentType,
//     and the detected MIME type is compared against the list of allowed content types.
//   - If the detected MIME type is not in the allowed list, an error is returned.
//
// The method implements the io.Reader interface.
func (mr *MIMEReader) Read(p []byte) (n int, err error) {
	if mr.err != nil {
		// Previous calls failed, return the error again.
		return 0, mr.err
	}

	if mr.mimetype != "" {
		// Read from the underlying reader till the end
		n, err = mr.r.Read(p)
		mr.read += int64(n)
		return n, err
	}

	// Read from the underlying reader
	n, err = mr.r.Read(p)
	mr.read += int64(n)

	mr.buf.Write(p[0:n])

	switch {
	case err == io.EOF || (mr.read >= sniffLen && mr.buf.Len() > 0):
		defer mr.buf.Reset()
		mtype := mimetype.Detect(mr.buf.Bytes())
		mt, _, _ := mime.ParseMediaType(mtype.String())
		if mt == "" || !slices.Contains(mr.allowedContentTypes, mt) {
			mr.err = errkit.WithFields(ErrInvalidContentType, errkit.Fields{
				"expected": mr.allowedContentTypes,
				"detected": mtype,
				"parsed":   mt,
			})
			log.Printf("=================== Invalid MIME: %s", mr.err.Error())
			return n, mr.err
		}
		mr.mimetype = mt
	case err != nil:
		mr.buf.Reset() // if we had an error, and it wasn't io.EOF, we should reset the buffer
		mr.err = err
	}

	return n, err
}

func (mr *MIMEReader) MIMEType() string {
	return mr.mimetype
}
