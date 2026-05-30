// Package inb is the single source of truth for the framing of an Inventario
// `.inb` backup archive (issue #534).
//
// An `.inb` file is an OUTER, uncompressed tar with exactly two members:
//
//	payload.tar.gz.sig   — the detached Ed25519 signature over the streaming
//	                       SHA-256 digest of payload.tar.gz (see backupsign).
//	payload.tar.gz       — gzip(level 3) of the inner tar (manifest.json +
//	                       per-location JSON + files/...). The actual backup.
//
// This follows the design owner's recipe — `tar(*.tar.gz + *.tar.gz.sig)` — and
// lets a backup be re-signed (e.g. after a signing-key rotation) by swapping
// only the `.sig` member while leaving payload.tar.gz byte-identical.
//
// # Memory
//
// The signature member is written FIRST so the reader can pull the tiny
// signature before it has to stream the (potentially huge) payload. Neither
// WriteContainer nor ReadContainer buffers the payload: WriteContainer streams
// it from an io.Reader with a known size, and ReadContainer returns it as a
// bounded io.Reader. The caller streams the payload through a hasher to verify
// and (only then) inflates it — typically spooling to a temp file so the whole
// archive never lands on the heap.
//
// ReadContainer is hardened against hostile archives: it enforces the
// signature-first ordering, size limits, and rejects non-regular members
// (symlinks/dirs/devices), unexpected names, and duplicates. It performs NO
// cryptographic verification — that is the caller's responsibility and MUST
// happen before the payload is inflated.
package inb

import (
	"archive/tar"
	"io"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
)

const (
	// SignatureName is the outer-tar member holding the detached signature. It
	// is written first to enable streaming verification.
	SignatureName = "payload.tar.gz.sig"
	// PayloadName is the outer-tar member holding the gzipped inner tar.
	PayloadName = "payload.tar.gz"
)

var (
	// ErrMissingSignature is returned when the archive has no signature member —
	// an unsigned backup, which restore refuses.
	ErrMissingSignature = errx.NewSentinel("backup archive is missing the payload.tar.gz.sig member")
	// ErrMissingPayload is returned when the archive has no payload member (not
	// a valid `.inb`, e.g. a legacy XML upload).
	ErrMissingPayload = errx.NewSentinel("backup archive is missing the payload.tar.gz member")
	// ErrBadOrder is returned when the payload precedes the signature, which
	// would force the verifier to buffer the whole payload before it can check
	// the signature. Canonical `.inb` archives are always signature-first.
	ErrBadOrder = errx.NewSentinel("backup archive members are out of order (signature must come first)")
	// ErrPayloadTooLarge is returned when payload.tar.gz exceeds the limit.
	ErrPayloadTooLarge = errx.NewSentinel("backup archive payload exceeds the maximum allowed size")
	// ErrSignatureTooLarge is returned when the signature member is implausibly
	// large (a 64-byte Ed25519 signature should never approach the limit).
	ErrSignatureTooLarge = errx.NewSentinel("backup archive signature member exceeds the maximum allowed size")
	// ErrUnexpectedMember is returned for a member that is neither expected name
	// or is a non-regular file (defence against tar smuggling / symlink tricks).
	ErrUnexpectedMember = errx.NewSentinel("backup archive contains an unexpected member")
	// ErrDuplicateMember is returned when an expected member appears twice — an
	// ambiguity trick (which copy did the signature cover?).
	ErrDuplicateMember = errx.NewSentinel("backup archive contains a duplicate member")
)

// Limits bounds the resources ReadContainer will consume. The signature is read
// fully into memory (tiny); the payload is never buffered by this package but is
// capped so a lying header cannot make the caller copy an unbounded stream.
type Limits struct {
	// MaxPayloadBytes caps payload.tar.gz. Default (DefaultLimits) is 4 GiB.
	MaxPayloadBytes int64
	// MaxSignatureBytes caps payload.tar.gz.sig. An Ed25519 signature is 64
	// bytes; the default leaves generous slack while still rejecting abuse.
	MaxSignatureBytes int64
}

// DefaultLimits returns conservative limits suitable for the restore path. The
// payload cap is intentionally generous (4 GiB) but bounded; operators with very
// large inventories can raise MaxPayloadBytes explicitly. Because the payload is
// streamed (not heap-buffered), the cap guards copy work, not allocation.
func DefaultLimits() Limits {
	return Limits{
		MaxPayloadBytes:   4 << 30, // 4 GiB
		MaxSignatureBytes: 4 << 10, // 4 KiB
	}
}

// WriteContainer writes the outer `.inb` tar to w: the signature member first,
// then the payload member streamed from payload (exactly payloadSize bytes).
// The payload is copied straight through — nothing is buffered.
func WriteContainer(w io.Writer, sig []byte, payload io.Reader, payloadSize int64) error {
	tw := tar.NewWriter(w)

	if err := tw.WriteHeader(&tar.Header{
		Name:     SignatureName,
		Mode:     0o600,
		Size:     int64(len(sig)),
		Typeflag: tar.TypeReg,
	}); err != nil {
		return errxtrace.Wrap("failed to write signature header", err)
	}
	if _, err := tw.Write(sig); err != nil {
		return errxtrace.Wrap("failed to write signature member", err)
	}

	if err := tw.WriteHeader(&tar.Header{
		Name:     PayloadName,
		Mode:     0o600,
		Size:     payloadSize,
		Typeflag: tar.TypeReg,
	}); err != nil {
		return errxtrace.Wrap("failed to write payload header", err)
	}
	if _, err := io.Copy(tw, payload); err != nil {
		return errxtrace.Wrap("failed to write payload member", err)
	}

	if err := tw.Close(); err != nil {
		return errxtrace.Wrap("failed to finalize backup container", err)
	}
	return nil
}

// ReadContainer reads the signature member (first) fully and returns the payload
// member (second) as a streaming io.Reader bounded to MaxPayloadBytes. It
// enforces signature-first ordering and rejects non-regular/unexpected/duplicate
// members and oversized members (by declared header size).
//
// The returned payload reader is valid only while r is being consumed; the
// caller MUST fully read it (typically copying it through a hasher to a temp
// file) and verify sig BEFORE inflating. Any members after payload are ignored —
// they are inert because nothing reads them and the signature covers only the
// payload.
func ReadContainer(r io.Reader, limits Limits) (sig []byte, payload io.Reader, err error) {
	tr := tar.NewReader(r)

	// Member 1 must be the signature.
	h1, err := tr.Next()
	if err == io.EOF {
		return nil, nil, ErrMissingSignature
	}
	if err != nil {
		return nil, nil, errxtrace.Wrap("failed to read backup container", err)
	}
	if h1.Typeflag != tar.TypeReg {
		return nil, nil, errx.Classify(ErrUnexpectedMember, errx.Attrs("name", h1.Name, "typeflag", h1.Typeflag))
	}
	switch h1.Name {
	case SignatureName:
		// ok
	case PayloadName:
		return nil, nil, ErrBadOrder
	default:
		return nil, nil, errx.Classify(ErrUnexpectedMember, errx.Attrs("name", h1.Name))
	}
	if h1.Size > limits.MaxSignatureBytes {
		return nil, nil, ErrSignatureTooLarge
	}
	sig, err = readLimited(tr, limits.MaxSignatureBytes, ErrSignatureTooLarge)
	if err != nil {
		return nil, nil, err
	}

	// Member 2 must be the payload.
	h2, err := tr.Next()
	if err == io.EOF {
		return nil, nil, ErrMissingPayload
	}
	if err != nil {
		return nil, nil, errxtrace.Wrap("failed to read backup container", err)
	}
	if h2.Typeflag != tar.TypeReg {
		return nil, nil, errx.Classify(ErrUnexpectedMember, errx.Attrs("name", h2.Name, "typeflag", h2.Typeflag))
	}
	switch h2.Name {
	case PayloadName:
		// ok
	case SignatureName:
		return nil, nil, ErrDuplicateMember
	default:
		return nil, nil, errx.Classify(ErrUnexpectedMember, errx.Attrs("name", h2.Name))
	}
	if h2.Size > limits.MaxPayloadBytes {
		return nil, nil, ErrPayloadTooLarge
	}

	// tar.Reader already bounds reads to the member's declared size; the extra
	// LimitReader is belt-and-suspenders against a backend that mis-reports.
	return sig, io.LimitReader(tr, limits.MaxPayloadBytes), nil
}

// readLimited reads up to maxBytes from r, returning tooLarge if the member
// would exceed the cap. It reads one byte past the limit to distinguish
// "exactly at the limit" from "over the limit".
func readLimited(r io.Reader, maxBytes int64, tooLarge error) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, maxBytes+1))
	if err != nil {
		return nil, errxtrace.Wrap("failed to read backup container member", err)
	}
	if int64(len(data)) > maxBytes {
		return nil, tooLarge
	}
	return data, nil
}
