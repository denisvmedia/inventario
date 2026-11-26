package export

import (
	"archive/tar"
	"context"
	"encoding/json"
	"io"
	"time"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/shopspring/decimal"
	"gocloud.dev/blob"

	"go.5x5.cz/inventario/models"
)

// gzipLevel is the compression level of a payload.tar.gz, the group backup's
// inner payload and the personal data export alike. Level 3 trades a little
// ratio for much faster compression, which matters because the archive is
// built synchronously in a worker and streamed straight to blob storage
// (#534).
const gzipLevel = 3

// writeTarMember writes data as the tar member name.
func writeTarMember(tw *tar.Writer, name string, data []byte) error {
	hdr := &tar.Header{
		Name:     name,
		Mode:     0o600,
		Size:     int64(len(data)),
		Typeflag: tar.TypeReg,
		ModTime:  time.Now(),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return errxtrace.Wrap("failed to write tar header", err, errx.Attrs("member", name))
	}
	if _, err := tw.Write(data); err != nil {
		return errxtrace.Wrap("failed to write tar member", err, errx.Attrs("member", name))
	}
	return nil
}

// streamTarMember streams the blob at blobKey into the tar member name
// without buffering it. size is the member size the header declares and has
// to be the blob's exact size. Returns the byte count written.
func streamTarMember(ctx context.Context, tw *tar.Writer, bucket *blob.Bucket, name, blobKey string, size int64) (int64, error) {
	reader, err := bucket.NewReader(ctx, blobKey, nil)
	if err != nil {
		return 0, errxtrace.Wrap("failed to open blob reader for file member", err, errx.Attrs("blob_key", blobKey))
	}
	defer reader.Close()

	hdr := &tar.Header{
		Name:     name,
		Mode:     0o600,
		Size:     size,
		Typeflag: tar.TypeReg,
		ModTime:  time.Now(),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return 0, errxtrace.Wrap("failed to write file member header", err, errx.Attrs("member", name))
	}
	n, err := io.Copy(tw, reader)
	if err != nil {
		return 0, errxtrace.Wrap("failed to stream file member", err, errx.Attrs("member", name))
	}
	return n, nil
}

// fileMemberName returns the basename to use for a file inside the archive:
// its Path (user-facing name without extension) plus its extension.
func fileMemberName(file *models.FileEntity) string {
	name := file.Path
	if name == "" {
		name = file.UUID
	}
	return name + file.Ext
}

// ptrDecimalString renders a nullable decimal (the three prices, SalePrice,
// AcquisitionPrice). It preserves a legitimate zero value: the meaningful
// distinction for these columns is nil (unknown) vs set, so a non-nil pointer
// holding 0 is emitted as "0" and only a nil pointer yields "" (the omitempty
// JSON field then disappears). The restore side mirrors this: empty → nil, any
// non-empty value → a set pointer.
func ptrDecimalString(d *decimal.Decimal) string {
	if d == nil {
		return ""
	}
	return d.String()
}

// urlStrings flattens a slice of *URL to their string forms.
func urlStrings(urls models.ValuerSlice[*models.URL]) []string {
	if len(urls) == 0 {
		return nil
	}
	out := make([]string, 0, len(urls))
	for _, u := range urls {
		if u != nil {
			out = append(out, u.String())
		}
	}
	return out
}

// encodeJSONMember encodes v as a JSON member is written.
func encodeJSONMember(name string, v any) ([]byte, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, errxtrace.Wrap("failed to marshal JSON member", err, errx.Attrs("member", name))
	}
	return data, nil
}
