package services

import (
	"cmp"
	"context"
	"slices"
	"strings"
	"time"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

// FailedJobService is the one place the failed-jobs surfaces go through
// (#1307): the system-admin page, the group settings view, the CLI and the
// depth gauge. It merges the per-type adapters of the FailedJobRegistry into
// one list and checks a type before it reaches them. It does not audit: the
// caller knows who acted and on which plane.
type FailedJobService struct {
	jobs  registry.FailedJobRegistry
	clock func() time.Time
}

// NewFailedJobService returns a service over the registry.
func NewFailedJobService(jobs registry.FailedJobRegistry) *FailedJobService {
	return &FailedJobService{jobs: jobs, clock: time.Now}
}

// FailedJobQuery selects a page of failed jobs.
type FailedJobQuery struct {
	// Type limits the list to one job type; empty lists every type.
	Type  models.WorkerType
	Scope registry.FailedJobScope
	// Offset and Limit page the merged list. A limit of zero or less
	// returns every job from the offset on.
	Offset int
	Limit  int
}

// FailedJobPage is one page of failed jobs, newest failure first, and how
// many jobs the query matches in all.
type FailedJobPage struct {
	Jobs  []*models.FailedJob
	Total int
}

// FailedJobTypesIn returns the types whose failed jobs the scope reaches: in
// a group, only the types whose jobs belong to a group.
func FailedJobTypesIn(scope registry.FailedJobScope) []models.WorkerType {
	if scope.GroupID != "" {
		return models.FailedJobGroupTypes()
	}
	return models.FailedJobTypes()
}

// IsFailedJobTypeIn reports whether the scope reaches failed jobs of the
// type. A group scope does not reach a personal data export, which belongs
// to a user.
func IsFailedJobTypeIn(scope registry.FailedJobScope, jobType models.WorkerType) bool {
	if scope.GroupID != "" {
		return models.IsFailedJobGroupType(jobType)
	}
	return models.IsFailedJobType(jobType)
}

func (s *FailedJobService) checkType(jobType models.WorkerType, scope registry.FailedJobScope) error {
	if !IsFailedJobTypeIn(scope, jobType) {
		return errxtrace.Classify(registry.ErrUnknownFailedJobType, errx.Attrs("job_type", string(jobType)))
	}
	return nil
}

// List returns the page of failed jobs the query selects.
func (s *FailedJobService) List(ctx context.Context, q FailedJobQuery) (FailedJobPage, error) {
	types := FailedJobTypesIn(q.Scope)
	if q.Type != "" {
		if err := s.checkType(q.Type, q.Scope); err != nil {
			return FailedJobPage{}, err
		}
		types = []models.WorkerType{q.Type}
	}
	offset := max(q.Offset, 0)
	// Each type lists at most the jobs up to the end of the page, so the
	// merge never reads a type's whole backlog for the first page.
	perType := 0
	if q.Limit > 0 {
		perType = offset + q.Limit
	}

	var page FailedJobPage
	var all []*models.FailedJob
	for _, jobType := range types {
		n, err := s.jobs.CountFailed(ctx, jobType, q.Scope)
		if err != nil {
			return FailedJobPage{}, errxtrace.Wrap("failed to count failed jobs", err)
		}
		page.Total += n
		if n == 0 {
			continue
		}
		jobs, err := s.jobs.ListFailed(ctx, jobType, q.Scope, perType)
		if err != nil {
			return FailedJobPage{}, errxtrace.Wrap("failed to list failed jobs", err)
		}
		all = append(all, jobs...)
	}
	slices.SortStableFunc(all, compareFailedJobs)

	if offset >= len(all) {
		page.Jobs = []*models.FailedJob{}
		return page, nil
	}
	all = all[offset:]
	if q.Limit > 0 && len(all) > q.Limit {
		all = all[:q.Limit]
	}
	page.Jobs = all
	return page, nil
}

// compareFailedJobs orders newest failure first, a job without a failure
// time last, then by type and id so a page boundary is stable.
func compareFailedJobs(a, b *models.FailedJob) int {
	switch {
	case a.FailedAt == nil && b.FailedAt != nil:
		return 1
	case a.FailedAt != nil && b.FailedAt == nil:
		return -1
	case a.FailedAt != nil && !a.FailedAt.Equal(*b.FailedAt):
		return b.FailedAt.Compare(*a.FailedAt)
	}
	if c := cmp.Compare(a.Type, b.Type); c != 0 {
		return c
	}
	return cmp.Compare(a.ID, b.ID)
}

// Get returns one failed job in the scope.
func (s *FailedJobService) Get(ctx context.Context, jobType models.WorkerType, scope registry.FailedJobScope, id string) (*models.FailedJob, error) {
	if err := s.checkType(jobType, scope); err != nil {
		return nil, err
	}
	return s.jobs.GetFailed(ctx, jobType, scope, id)
}

// Retry puts the failed job back in its queue and returns the job as it was
// before the retry, for the caller's audit record.
// ErrFailedJobNotRetryable for a type that can only be discarded.
func (s *FailedJobService) Retry(ctx context.Context, jobType models.WorkerType, scope registry.FailedJobScope, id string) (*models.FailedJob, error) {
	job, err := s.Get(ctx, jobType, scope, id)
	if err != nil {
		return nil, err
	}
	if !models.FailedJobRetryable(jobType) {
		return job, errxtrace.Classify(registry.ErrFailedJobNotRetryable, errx.Attrs("job_type", string(jobType)))
	}
	if err := s.jobs.Retry(ctx, jobType, scope, id, s.clock()); err != nil {
		return job, err
	}
	return job, nil
}

// Discard marks the failed job discarded and returns it, for the caller's
// audit record. The job keeps its row and its error.
func (s *FailedJobService) Discard(ctx context.Context, jobType models.WorkerType, scope registry.FailedJobScope, id string) (*models.FailedJob, error) {
	job, err := s.Get(ctx, jobType, scope, id)
	if err != nil {
		return nil, err
	}
	if err := s.jobs.Discard(ctx, jobType, scope, id, s.clock()); err != nil {
		return job, err
	}
	return job, nil
}

// FailedJobBulkQuery selects the failed jobs a bulk discard takes (#2946):
// the jobs in the scope that failed before Scope.FailedBefore, of one type
// or of every type.
type FailedJobBulkQuery struct {
	// Type limits the query to one job type; empty takes every type.
	Type models.WorkerType
	// Scope must set FailedBefore, to a time that is not in the future.
	Scope registry.FailedJobScope
}

// FailedJobCounts is how many failed jobs a bulk query took, in all and per
// type. ByType leaves out the types it took none of.
type FailedJobCounts struct {
	Total  int
	ByType map[string]int
}

func (c *FailedJobCounts) add(jobType models.WorkerType, n int) {
	if n == 0 {
		return
	}
	c.Total += n
	c.ByType[string(jobType)] = n
}

// bulkTypes checks the query and returns the types it takes. A time in the
// future is refused: the jobs failing between a dry run and the discard
// would then be discarded without having been counted.
func (s *FailedJobService) bulkTypes(q FailedJobBulkQuery) ([]models.WorkerType, error) {
	switch {
	case q.Scope.FailedBefore.IsZero():
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field", "failed_before"))
	case q.Scope.FailedBefore.After(s.clock()):
		return nil, errxtrace.Classify(registry.ErrInvalidInput, errx.Attrs("field", "failed_before", "reason", "in the future"))
	case q.Type == "":
		return FailedJobTypesIn(q.Scope), nil
	}
	if err := s.checkType(q.Type, q.Scope); err != nil {
		return nil, err
	}
	return []models.WorkerType{q.Type}, nil
}

// CountBulk counts the failed jobs DiscardBulk would discard: the dry run.
func (s *FailedJobService) CountBulk(ctx context.Context, q FailedJobBulkQuery) (FailedJobCounts, error) {
	types, err := s.bulkTypes(q)
	if err != nil {
		return FailedJobCounts{}, err
	}
	counts := FailedJobCounts{ByType: map[string]int{}}
	for _, jobType := range types {
		n, err := s.jobs.CountFailed(ctx, jobType, q.Scope)
		if err != nil {
			return FailedJobCounts{}, errxtrace.Wrap("failed to count failed jobs", err, errx.Attrs("job_type", string(jobType)))
		}
		counts.add(jobType, n)
	}
	return counts, nil
}

// DiscardBulk marks discarded every failed job the query selects and
// returns how many it discarded. A job that someone retried or discarded in
// the meantime is not counted. Each type is discarded on its own: when one
// fails, the counts of the types before it come back with the error.
func (s *FailedJobService) DiscardBulk(ctx context.Context, q FailedJobBulkQuery) (FailedJobCounts, error) {
	types, err := s.bulkTypes(q)
	if err != nil {
		return FailedJobCounts{}, err
	}
	now := s.clock()
	counts := FailedJobCounts{ByType: map[string]int{}}
	for _, jobType := range types {
		n, err := s.jobs.DiscardFailed(ctx, jobType, q.Scope, now)
		if err != nil {
			return counts, errxtrace.Wrap("failed to discard failed jobs", err, errx.Attrs("job_type", string(jobType)))
		}
		counts.add(jobType, n)
	}
	return counts, nil
}

// ParseFailedBefore reads the time a bulk discard takes the jobs before: a
// date, which stands for its midnight UTC, or an RFC 3339 time.
func ParseFailedBefore(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if t, err := time.Parse(time.DateOnly, raw); err == nil {
		return t, nil
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, errxtrace.Classify(registry.ErrInvalidInput,
			errx.Attrs("field", "failed_before", "reason", "not a date (2006-01-02) or an RFC 3339 time"))
	}
	return t.UTC(), nil
}

// Depths counts the failed jobs nobody retried or discarded, per type,
// across every tenant. It feeds the inventario_jobs_dead_letter_depth
// gauge and must not serve a request.
func (s *FailedJobService) Depths(ctx context.Context) (map[string]int, error) {
	depths := make(map[string]int)
	for _, jobType := range models.FailedJobTypes() {
		n, err := s.jobs.CountFailed(ctx, jobType, registry.FailedJobScope{})
		if err != nil {
			return nil, errxtrace.Wrap("failed to count failed jobs", err, errx.Attrs("job_type", string(jobType)))
		}
		depths[string(jobType)] = n
	}
	return depths, nil
}
