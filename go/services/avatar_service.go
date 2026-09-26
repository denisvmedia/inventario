package services

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	_ "image/gif" // registers the GIF decoder for image.Decode
	"image/jpeg"
	_ "image/png" // registers the PNG decoder for image.Decode
	"io"
	"path"
	"strconv"
	"strings"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
	"gocloud.dev/blob"

	"go.5x5.cz/inventario/internal/mimekit"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/services/imageprocessor"
)

const (
	// AvatarMaxUploadBytes caps what will be read from an upload. The mock's
	// hint says 2 MB and this is that number; a 512px JPEG is a few tens of KB,
	// so the cap is about refusing a hostile body rather than a large photo.
	AvatarMaxUploadBytes int64 = 2 << 20
	// AvatarSize is the stored edge length. Rendered at 64px in the profile and
	// 32px in member lists, 512 leaves room for a high-density display without
	// storing originals.
	AvatarSize = 512
	// avatarPrefix is the key prefix avatars live under, kept away from the
	// commodity file keys so a bucket listing is readable and a lifecycle rule
	// can treat them differently.
	avatarPrefix = "avatars"
	// avatarContentType is what every stored avatar is, whatever arrived:
	// decoding to an image and re-encoding normalises the format.
	avatarContentType = "image/jpeg"
	avatarExt         = ".jpg"
)

// avatarUploadContentTypes is what an upload may be. It is narrower than
// mimekit.ImageContentTypes() on purpose, and narrower than what image.Decode
// would accept: the profile page promises "JPG or PNG", so a GIF or a WebP is
// refused with that reason rather than failing later in the decoder with a
// message about an unknown format.
var avatarUploadContentTypes = []string{"image/jpeg", "image/png"}

// AvatarService stores and removes user profile photos (#1382).
//
// Everything that arrives is decoded, center-cropped, scaled and re-encoded as
// JPEG. That is not only about size: decoding to an image.Image drops every
// metadata block the file carried, so the EXIF GPS tag a phone camera writes
// cannot survive into a photo other group members can fetch. Re-encoding is the
// EXIF strip, rather than a separate step that could be forgotten.
type AvatarService struct {
	factorySet     *registry.FactorySet
	uploadLocation string
	processor      *imageprocessor.ImageProcessor
}

// NewAvatarService constructs the service.
func NewAvatarService(factorySet *registry.FactorySet, uploadLocation string) *AvatarService {
	return &AvatarService{
		factorySet:     factorySet,
		uploadLocation: uploadLocation,
		processor:      imageprocessor.NewDefault(),
	}
}

// Store reads an uploaded image, normalises it and records it as the user's
// avatar, returning the stored key.
//
// The previous avatar is deleted after the new one is recorded, in that order:
// a failure between the two leaves an unreferenced object, which costs storage,
// while the other order would leave the row pointing at an object that is gone.
func (s *AvatarService) Store(ctx context.Context, userID string, src io.Reader) (string, error) {
	if userID == "" {
		return "", errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "UserID"))
	}

	user, err := s.factorySet.UserRegistry.Get(ctx, userID)
	if err != nil {
		return "", errxtrace.Wrap("failed to load the user", err)
	}

	encoded, err := s.normalize(src)
	if err != nil {
		return "", err
	}

	key := avatarKey(userID, nextAvatarRevision(user.AvatarPath))
	if err := s.write(ctx, key, encoded); err != nil {
		return "", err
	}

	previous := user.AvatarPath
	user.AvatarPath = &key
	if _, err := s.factorySet.UserRegistry.Update(ctx, *user); err != nil {
		// The object is written but unreferenced. Remove it rather than leaving
		// litter behind a failure the caller will retry.
		s.deleteQuietly(ctx, key)
		return "", errxtrace.Wrap("failed to record the avatar", err)
	}
	if previous != nil && *previous != "" && *previous != key {
		s.deleteQuietly(ctx, *previous)
	}
	return key, nil
}

// Remove clears the user's avatar and deletes the stored object. It is
// idempotent: a user with no avatar is not an error.
func (s *AvatarService) Remove(ctx context.Context, userID string) error {
	if userID == "" {
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "UserID"))
	}

	user, err := s.factorySet.UserRegistry.Get(ctx, userID)
	if err != nil {
		return errxtrace.Wrap("failed to load the user", err)
	}
	if user.AvatarPath == nil || *user.AvatarPath == "" {
		return nil
	}

	previous := *user.AvatarPath
	user.AvatarPath = nil
	if _, err := s.factorySet.UserRegistry.Update(ctx, *user); err != nil {
		return errxtrace.Wrap("failed to clear the avatar", err)
	}
	s.deleteQuietly(ctx, previous)
	return nil
}

// Open returns the stored avatar for reading, along with its content type.
func (s *AvatarService) Open(ctx context.Context, userID string) (io.ReadCloser, string, error) {
	user, err := s.factorySet.UserRegistry.Get(ctx, userID)
	if err != nil {
		return nil, "", errxtrace.Wrap("failed to load the user", err)
	}
	if user.AvatarPath == nil || *user.AvatarPath == "" {
		return nil, "", errxtrace.Classify(registry.ErrNotFound, errx.Attrs("entity_type", "Avatar", "user_id", userID))
	}

	b, err := blob.OpenBucket(ctx, s.uploadLocation)
	if err != nil {
		return nil, "", errxtrace.Wrap("failed to open bucket", err)
	}
	reader, err := b.NewReader(ctx, *user.AvatarPath, nil)
	if err != nil {
		_ = b.Close()
		return nil, "", errxtrace.Wrap("failed to read the avatar", err)
	}
	// The bucket outlives the reader, so it is closed with it.
	return &bucketBoundReader{ReadCloser: reader, bucket: b}, avatarContentType, nil
}

// bucketBoundReader closes the bucket the reader came from.
type bucketBoundReader struct {
	io.ReadCloser
	bucket *blob.Bucket
}

func (r *bucketBoundReader) Close() error {
	err := r.ReadCloser.Close()
	if closeErr := r.bucket.Close(); err == nil {
		err = closeErr
	}
	return err
}

var (
	// ErrAvatarNotAnImage is returned when the upload is not one of the image
	// types the avatar endpoints accept, or is not decodable as one.
	ErrAvatarNotAnImage = errors.New("avatar: not a supported image")
	// ErrAvatarTooLarge is returned when the upload exceeds AvatarMaxUploadBytes.
	ErrAvatarTooLarge = errors.New("avatar: image is too large")
)

// normalize reads at most AvatarMaxUploadBytes, checks that the bytes really are
// an image of an accepted type, and returns a square JPEG.
//
// The type comes from sniffing the content, never from the client's declared
// content type or the filename: both are attacker-chosen, and the whole point of
// the check is to refuse what is not an image.
func (s *AvatarService) normalize(src io.Reader) ([]byte, error) {
	// The MIMEReader sniffs the content and refuses anything outside the avatar
	// allowlist, the same gate the file uploads use. The declared content type
	// and the filename decide nothing: both are attacker-chosen.
	//
	// The decode below would also reject most non-images, so this is not the
	// only line of defence — what it adds is refusing a format we decline to
	// accept even though Go can read it, and saying so before spending a decode
	// on a body that was never going to be stored.
	//
	// One byte past the cap, so a body exactly at the limit is accepted and one
	// over it is refused rather than silently truncated into a broken image.
	limited := io.LimitReader(src, AvatarMaxUploadBytes+1)
	raw, err := io.ReadAll(mimekit.NewMIMEReader(limited, avatarUploadContentTypes))
	switch {
	case errors.Is(err, mimekit.ErrInvalidContentType):
		return nil, ErrAvatarNotAnImage
	case err != nil:
		return nil, errxtrace.Wrap("failed to read the upload", err)
	}
	if int64(len(raw)) > AvatarMaxUploadBytes {
		return nil, errxtrace.Wrap(fmt.Sprintf("avatar exceeds %d bytes", AvatarMaxUploadBytes), ErrAvatarTooLarge)
	}

	decoded, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		// Sniffed as an image but undecodable: a truncated or malformed file.
		return nil, errxtrace.Wrap("failed to decode the image", ErrAvatarNotAnImage)
	}

	var out bytes.Buffer
	if err := jpeg.Encode(&out, s.processor.CropSquare(decoded, AvatarSize), &jpeg.Options{Quality: 90}); err != nil {
		return nil, errxtrace.Wrap("failed to encode the avatar", err)
	}
	return out.Bytes(), nil
}

func (s *AvatarService) write(ctx context.Context, key string, data []byte) error {
	b, err := blob.OpenBucket(ctx, s.uploadLocation)
	if err != nil {
		return errxtrace.Wrap("failed to open bucket", err)
	}
	defer b.Close()

	w, err := b.NewWriter(ctx, key, &blob.WriterOptions{ContentType: avatarContentType})
	if err != nil {
		return errxtrace.Wrap("failed to open the avatar for writing", err)
	}
	if _, err := w.Write(data); err != nil {
		_ = w.Close()
		return errxtrace.Wrap("failed to write the avatar", err)
	}
	if err := w.Close(); err != nil {
		return errxtrace.Wrap("failed to finish writing the avatar", err)
	}
	return nil
}

// deleteQuietly removes an object and logs nothing on failure beyond the error
// the caller may ignore: an avatar left behind is storage, not a fault the user
// should see.
func (s *AvatarService) deleteQuietly(ctx context.Context, key string) {
	b, err := blob.OpenBucket(ctx, s.uploadLocation)
	if err != nil {
		return
	}
	defer b.Close()
	_ = b.Delete(ctx, key)
}

// avatarKey builds the stored key for a revision.
func avatarKey(userID string, revision int) string {
	return path.Join(avatarPrefix, userID, strconv.Itoa(revision)+avatarExt)
}

// nextAvatarRevision reads the revision out of the current key and returns the
// next one, starting at 1 when there is no current avatar or its key does not
// carry a number.
//
// Deriving it from the key rather than storing a counter keeps the two from
// disagreeing: the key is the only thing that has to be unique.
func nextAvatarRevision(current *string) int {
	if current == nil {
		return 1
	}
	base := strings.TrimSuffix(path.Base(*current), avatarExt)
	revision, err := strconv.Atoi(base)
	if err != nil || revision < 1 {
		return 1
	}
	return revision + 1
}
