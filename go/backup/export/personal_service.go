package export

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path"
	"strings"
	"time"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
	"gocloud.dev/blob"
	"gocloud.dev/gcerrors"

	"go.5x5.cz/inventario/internal/blobkeys"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

var (
	// ErrPersonalDataExportRateLimited is what a request made within
	// models.PersonalDataExportRequestInterval of the previous one fails
	// with. errors.As finds a *PersonalDataExportRateLimitedError carrying
	// the time a new request is allowed.
	ErrPersonalDataExportRateLimited = errx.NewSentinel("a personal data export can be requested once a day")
	// ErrPersonalDataExportNotReady is what a download of an export without
	// a finished archive fails with: one still being built, one that failed
	// or one past its expiry.
	ErrPersonalDataExportNotReady = errx.NewSentinel("the personal data export is not ready to download")
)

// PersonalDataExportRateLimitedError says when the user may request a
// personal data export again.
type PersonalDataExportRateLimitedError struct {
	RetryAt time.Time
}

func (e *PersonalDataExportRateLimitedError) Error() string {
	return fmt.Sprintf("%s; the next request is allowed at %s", ErrPersonalDataExportRateLimited, e.RetryAt.UTC().Format(time.RFC3339))
}

// Is makes errors.Is(err, ErrPersonalDataExportRateLimited) hold.
func (*PersonalDataExportRateLimitedError) Is(target error) bool {
	return target == ErrPersonalDataExportRateLimited
}

// personalOrphanGrace is how old an archive whose row is gone has to be
// before the sweep deletes it. A build writes the archive before it records
// it on the row; the grace keeps the sweep away from one being written.
const personalOrphanGrace = time.Hour

// personalStalledBuildAfter is how long a build may run before the worker
// takes its process for dead and queues the export again. A build streams
// the user's files from blob storage, so it is generous; a build still
// running after it is built a second time to the same key.
const personalStalledBuildAfter = time.Hour

// personalExpireBatch is how many expired exports one pass deletes.
const personalExpireBatch = 100

// PersonalDataExportNotifier tells a user their personal data export is
// ready.
type PersonalDataExportNotifier interface {
	PersonalDataExportReady(ctx context.Context, user *models.User, export *models.PersonalDataExport) error
}

// PersonalDataExportService requests, builds, serves and expires personal
// data exports (#2177). The export worker builds the pending ones and deletes
// the expired ones; the API requests, lists and downloads them.
type PersonalDataExportService struct {
	factorySet     *registry.FactorySet
	uploadLocation string
	notifier       PersonalDataExportNotifier
	now            func() time.Time
}

// PersonalDataExportOption customizes a PersonalDataExportService.
type PersonalDataExportOption func(*PersonalDataExportService)

// WithPersonalDataExportNotifier sets who tells a user their export is ready.
// Without one nobody is told.
func WithPersonalDataExportNotifier(n PersonalDataExportNotifier) PersonalDataExportOption {
	return func(s *PersonalDataExportService) { s.notifier = n }
}

// WithPersonalDataExportClock replaces the clock, for tests.
func WithPersonalDataExportClock(now func() time.Time) PersonalDataExportOption {
	return func(s *PersonalDataExportService) {
		if now != nil {
			s.now = now
		}
	}
}

// NewPersonalDataExportService returns a service storing archives in the
// bucket at uploadLocation.
func NewPersonalDataExportService(factorySet *registry.FactorySet, uploadLocation string, opts ...PersonalDataExportOption) *PersonalDataExportService {
	s := &PersonalDataExportService{
		factorySet:     factorySet,
		uploadLocation: uploadLocation,
		now:            time.Now,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (s *PersonalDataExportService) clock() time.Time {
	return s.now().UTC().Truncate(time.Microsecond)
}

// Request records a request for user's personal data export, which the
// export worker builds. A request within a day of the previous one fails
// with a *PersonalDataExportRateLimitedError.
func (s *PersonalDataExportService) Request(ctx context.Context, user *models.User) (*models.PersonalDataExport, error) {
	if user == nil || user.ID == "" || user.TenantID == "" {
		return nil, errx.NewSentinel("a user is required to request a personal data export")
	}
	now := s.clock()
	export := models.PersonalDataExport{
		TenantUserAwareEntityID: models.TenantUserAwareEntityID{TenantID: user.TenantID, UserID: user.ID},
		CreatedAt:               now,
		ExpiresAt:               now.Add(models.PersonalDataExportRetention),
	}
	created, blocking, err := s.factorySet.PersonalDataExportRegistry.CreateUnlessRequestedSince(ctx, export, now.Add(-models.PersonalDataExportRequestInterval))
	if err != nil {
		return nil, err
	}
	if blocking != nil {
		return nil, &PersonalDataExportRateLimitedError{RetryAt: blocking.CreatedAt.Add(models.PersonalDataExportRequestInterval).UTC()}
	}
	return created, nil
}

// List returns the user's exports, newest first, leaving out those past
// their expiry the worker has not deleted yet.
func (s *PersonalDataExportService) List(ctx context.Context, userID string) ([]*models.PersonalDataExport, error) {
	rows, err := s.factorySet.PersonalDataExportRegistry.ListByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	now := s.clock()
	out := make([]*models.PersonalDataExport, 0, len(rows))
	for _, r := range rows {
		if !r.IsExpired(now) {
			out = append(out, r)
		}
	}
	return out, nil
}

// NextRequestAt returns when the user may request an export again, or the
// zero time when they may now.
func (s *PersonalDataExportService) NextRequestAt(exports []*models.PersonalDataExport) time.Time {
	now := s.clock()
	for _, e := range exports {
		if at := e.CreatedAt.Add(models.PersonalDataExportRequestInterval); at.After(now) {
			return at.UTC()
		}
	}
	return time.Time{}
}

// Get returns the user's export, ErrNotFound for an id that is not theirs or
// is past its expiry.
func (s *PersonalDataExportService) Get(ctx context.Context, userID, id string) (*models.PersonalDataExport, error) {
	export, err := s.factorySet.PersonalDataExportRegistry.GetByUser(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	if export.IsExpired(s.clock()) {
		return nil, errxtrace.Classify(registry.ErrNotFound, errx.Attrs("entity_type", "PersonalDataExport", "entity_id", id))
	}
	return export, nil
}

// Open returns the user's export and a reader of its archive. The caller
// closes the reader.
func (s *PersonalDataExportService) Open(ctx context.Context, userID, id string) (*models.PersonalDataExport, io.ReadCloser, error) {
	export, err := s.Get(ctx, userID, id)
	if err != nil {
		return nil, nil, err
	}
	if !export.IsDownloadable(s.clock()) {
		return nil, nil, ErrPersonalDataExportNotReady
	}
	bucket, err := blob.OpenBucket(ctx, s.uploadLocation)
	if err != nil {
		return nil, nil, errxtrace.Wrap("failed to open blob bucket", err)
	}
	reader, err := bucket.NewReader(ctx, export.BlobKey, nil)
	if err != nil {
		_ = bucket.Close()
		if gcerrors.Code(err) == gcerrors.NotFound {
			return nil, nil, ErrPersonalDataExportNotReady
		}
		return nil, nil, errxtrace.Wrap("failed to open the archive", err)
	}
	return export, &bucketReadCloser{Reader: reader, bucket: bucket}, nil
}

// bucketReadCloser closes the bucket with the reader.
type bucketReadCloser struct {
	*blob.Reader
	bucket *blob.Bucket
}

func (r *bucketReadCloser) Close() error {
	return errors.Join(r.Reader.Close(), r.bucket.Close())
}

// ArchiveName is the file name a download of export is saved under.
func ArchiveName(export *models.PersonalDataExport) string {
	return "inventario-personal-data-" + export.CreatedAt.UTC().Format("2006-01-02") + ".tar.gz"
}

// Process builds the archive of an export the caller claimed and records the
// result on the row. A build that fails marks the row failed; the user can
// request again the next day.
func (s *PersonalDataExportService) Process(ctx context.Context, id string) error {
	reg := s.factorySet.PersonalDataExportRegistry
	export, err := reg.Get(ctx, id)
	if err != nil {
		return errxtrace.Wrap("failed to get personal data export", err)
	}
	user, err := s.factorySet.UserRegistry.Get(ctx, export.UserID)
	if err != nil {
		return s.fail(ctx, export, errxtrace.Wrap("failed to get the export's user", err))
	}
	if !user.IsActive {
		return s.fail(ctx, export, errx.NewSentinel("the account is not active"))
	}

	key := blobkeys.BuildPersonalDataExportBlobKey(export.TenantID, export.ID)
	size, stats, err := s.build(ctx, user, export, key)
	if err != nil {
		if ctx.Err() != nil {
			// The worker is stopping, for a deploy or a shutdown. The build
			// says nothing about the export, so it goes back to the queue and
			// the next worker builds it.
			return s.requeue(ctx, export, err)
		}
		return s.fail(ctx, export, err)
	}

	now := s.clock()
	export.Status = models.PersonalDataExportStatusCompleted
	export.BlobKey = key
	export.SizeBytes = size
	export.GroupCount = stats.GroupCount
	export.ItemCount = stats.ItemCount
	export.FileCount = stats.FileCount
	export.ErrorMessage = ""
	export.CompletedAt = new(now)
	export.ExpiresAt = now.Add(models.PersonalDataExportRetention)
	updated, err := reg.Update(ctx, *export)
	if err != nil {
		// Most likely the row went with the account while the archive was
		// built. Nothing lists an archive without a row, so it goes too.
		if bucket, openErr := blob.OpenBucket(ctx, s.uploadLocation); openErr == nil {
			_ = deleteBlob(ctx, bucket, key)
			_ = bucket.Close()
		}
		return errxtrace.Wrap("failed to record the finished personal data export", err)
	}

	if s.notifier != nil {
		if err := s.notifier.PersonalDataExportReady(ctx, user, updated); err != nil {
			// The archive is listed in Account settings either way.
			slog.WarnContext(ctx, "Failed to tell the user their personal data export is ready", "export_id", export.ID, "error", err)
		}
	}
	return nil
}

// build writes the archive to key and returns its size. A build that fails
// leaves nothing at key: the write is aborted by canceling its context.
// personalBuildAttempts is how many times a build starts over because a file
// was deleted while it ran.
const personalBuildAttempts = 3

// build writes the archive to key. A file deleted after the build planned the
// archive makes it start over, so the manifest and the group files list the
// file as missing instead of naming bytes the archive lacks.
func (s *PersonalDataExportService) build(ctx context.Context, user *models.User, export *models.PersonalDataExport, key string) (int64, PersonalStatistics, error) {
	bucket, err := blob.OpenBucket(ctx, s.uploadLocation)
	if err != nil {
		return 0, PersonalStatistics{}, errxtrace.Wrap("failed to open blob bucket", err)
	}
	defer bucket.Close()

	for attempt := 1; ; attempt++ {
		size, stats, err := s.buildOnce(ctx, bucket, user, export, key)
		if errors.Is(err, errPersonalFileVanished) && attempt < personalBuildAttempts {
			slog.InfoContext(ctx, "A file was deleted during a personal data export build, starting over",
				"export_id", export.ID, "attempt", attempt, "error", err)
			continue
		}
		return size, stats, err
	}
}

func (s *PersonalDataExportService) buildOnce(ctx context.Context, bucket *blob.Bucket, user *models.User, export *models.PersonalDataExport, key string) (int64, PersonalStatistics, error) {
	writeCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	writer, err := bucket.NewWriter(writeCtx, key, &blob.WriterOptions{
		ContentType:        "application/gzip",
		ContentDisposition: `attachment; filename="` + ArchiveName(export) + `"`,
	})
	if err != nil {
		return 0, PersonalStatistics{}, errxtrace.Wrap("failed to create blob writer", err)
	}
	counter := &countingWriter{w: writer}
	stats, err := writePersonalDataExport(ctx, s.factorySet, bucket, user, export.ID, export.CreatedAt, counter)
	if err != nil {
		cancel()
		_ = writer.Close()
		return 0, PersonalStatistics{}, err
	}
	if err := writer.Close(); err != nil {
		return 0, PersonalStatistics{}, errxtrace.Wrap("failed to store the archive", err)
	}
	return counter.n, stats, nil
}

type countingWriter struct {
	w io.Writer
	n int64
}

func (c *countingWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.n += int64(n)
	return n, err
}

// Claim moves a pending export to in_progress for the caller, recording the
// start of its build; see registry.PersonalDataExportRegistry.ClaimPending.
func (s *PersonalDataExportService) Claim(ctx context.Context, id string) (bool, error) {
	return s.factorySet.PersonalDataExportRegistry.ClaimPending(ctx, id, s.clock())
}

// RequeueStalled queues again the exports whose build started more than
// personalStalledBuildAfter ago and recorded no result, which happens when
// the process running it dies. The user still gets the copy they asked for,
// and the daily limit stays used by that one request.
func (s *PersonalDataExportService) RequeueStalled(ctx context.Context) (int, error) {
	n, err := s.factorySet.PersonalDataExportRegistry.RequeueStalled(ctx, s.clock().Add(-personalStalledBuildAfter))
	if err != nil {
		return 0, errxtrace.Wrap("failed to queue stalled personal data exports again", err)
	}
	return n, nil
}

// requeue returns an interrupted build to the queue. It writes with a
// context the stopping worker does not cancel.
func (s *PersonalDataExportService) requeue(ctx context.Context, export *models.PersonalDataExport, cause error) error {
	export.Status = models.PersonalDataExportStatusPending
	export.StartedAt = nil
	if _, err := s.factorySet.PersonalDataExportRegistry.Update(context.WithoutCancel(ctx), *export); err != nil {
		return errxtrace.Wrap("failed to queue the interrupted personal data export again", errors.Join(cause, err))
	}
	return errxtrace.Wrap("personal data export interrupted, queued again", cause)
}

// fail records a failed build. It writes with a context the worker does not
// cancel, so the row does not stay in_progress when the failure is the
// worker stopping.
func (s *PersonalDataExportService) fail(ctx context.Context, export *models.PersonalDataExport, cause error) error {
	ctx = context.WithoutCancel(ctx)
	export.Status = models.PersonalDataExportStatusFailed
	export.ErrorMessage = cause.Error()
	export.BlobKey = ""
	export.SizeBytes = 0
	if _, err := s.factorySet.PersonalDataExportRegistry.Update(ctx, *export); err != nil {
		return errxtrace.Wrap("failed to build personal data export", errors.Join(cause, err))
	}
	return errxtrace.Wrap("failed to build personal data export", cause)
}

// ExpireDue deletes the exports past their expiry and their archives, and
// returns how many it deleted. A row stays when its archive could not be
// deleted, so the next pass tries again.
func (s *PersonalDataExportService) ExpireDue(ctx context.Context) (int, error) {
	reg := s.factorySet.PersonalDataExportRegistry
	rows, err := reg.ListExpired(ctx, s.clock(), personalExpireBatch)
	if err != nil {
		return 0, errxtrace.Wrap("failed to list expired personal data exports", err)
	}
	if len(rows) == 0 {
		return 0, nil
	}
	bucket, err := blob.OpenBucket(ctx, s.uploadLocation)
	if err != nil {
		return 0, errxtrace.Wrap("failed to open blob bucket", err)
	}
	defer bucket.Close()

	deleted := 0
	var errs []error
	for _, row := range rows {
		if err := deleteBlob(ctx, bucket, blobkeys.BuildPersonalDataExportBlobKey(row.TenantID, row.ID)); err != nil {
			errs = append(errs, err)
			continue
		}
		if err := reg.Delete(ctx, row.ID); err != nil {
			errs = append(errs, err)
			continue
		}
		deleted++
	}
	return deleted, errors.Join(errs...)
}

func deleteBlob(ctx context.Context, bucket *blob.Bucket, key string) error {
	err := bucket.Delete(ctx, key)
	if err == nil || gcerrors.Code(err) == gcerrors.NotFound {
		return nil
	}
	return errxtrace.Wrap("failed to delete archive", err, errx.Attrs("blob_key", key))
}

// SweepOrphanedArchives deletes the archives whose export row is gone, older
// than an hour. A row goes without its archive when the user is deleted by an
// operator or with their tenant, paths that delete rows only.
func (s *PersonalDataExportService) SweepOrphanedArchives(ctx context.Context) (int, error) {
	bucket, err := blob.OpenBucket(ctx, s.uploadLocation)
	if err != nil {
		return 0, errxtrace.Wrap("failed to open blob bucket", err)
	}
	defer bucket.Close()

	cutoff := s.clock().Add(-personalOrphanGrace)
	deleted := 0
	tenants := bucket.List(&blob.ListOptions{Prefix: blobkeys.Prefix, Delimiter: "/"})
	for {
		tenant, err := tenants.Next(ctx)
		if errors.Is(err, io.EOF) {
			return deleted, nil
		}
		if err != nil {
			return deleted, errxtrace.Wrap("failed to list tenants in blob storage", err)
		}
		if !tenant.IsDir {
			continue
		}
		n, err := s.sweepTenant(ctx, bucket, tenant.Key+blobkeys.PersonalExportsSegment+"/", cutoff)
		deleted += n
		if err != nil {
			return deleted, err
		}
	}
}

func (s *PersonalDataExportService) sweepTenant(ctx context.Context, bucket *blob.Bucket, prefix string, cutoff time.Time) (int, error) {
	deleted := 0
	it := bucket.List(&blob.ListOptions{Prefix: prefix})
	for {
		obj, err := it.Next(ctx)
		if errors.Is(err, io.EOF) {
			return deleted, nil
		}
		if err != nil {
			return deleted, errxtrace.Wrap("failed to list personal data exports in blob storage", err)
		}
		if obj.IsDir || obj.ModTime.After(cutoff) {
			continue
		}
		id := strings.TrimSuffix(path.Base(obj.Key), ".tar.gz")
		_, err = s.factorySet.PersonalDataExportRegistry.Get(ctx, id)
		switch {
		case err == nil:
			continue
		case !errors.Is(err, registry.ErrNotFound):
			return deleted, err
		}
		if err := deleteBlob(ctx, bucket, obj.Key); err != nil {
			return deleted, err
		}
		deleted++
	}
}
