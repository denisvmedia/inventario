package admin

import (
	"context"
	"log/slog"
	"strings"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/services"
)

// Failed-job audit actions (#1307). The same literals as the admin HTTP
// plane, so one query finds every retry and discard whichever way it came.
const (
	auditActionFailedJobRetry   = "admin.failed_job_retry"
	auditActionFailedJobDiscard = "admin.failed_job_discard"
)

// FailedJobListRequest selects the failed jobs `inventario jobs failed list`
// prints.
type FailedJobListRequest struct {
	// Type limits the list to one job type; empty lists every type.
	Type string
	// Tenant is a tenant id or slug; empty lists every tenant.
	Tenant string
	// Limit caps the list; zero or less prints every job.
	Limit int
}

func (s *Service) failedJobs() (*services.FailedJobService, error) {
	if s.factorySet == nil || s.factorySet.FailedJobRegistry == nil {
		return nil, errxtrace.Classify(registry.ErrInvalidConfig, errx.Attrs("missing", "FailedJobRegistry"))
	}
	return services.NewFailedJobService(s.factorySet.FailedJobRegistry), nil
}

func parseFailedJobType(raw string) (models.WorkerType, error) {
	t := models.WorkerType(strings.TrimSpace(raw))
	if !models.IsFailedJobType(t) {
		return "", errxtrace.Classify(registry.ErrUnknownFailedJobType, errx.Attrs("job_type", raw))
	}
	return t, nil
}

// ListFailedJobs returns the failed jobs nobody retried or discarded,
// newest failure first, across every tenant unless the request names one.
func (s *Service) ListFailedJobs(ctx context.Context, req FailedJobListRequest) (services.FailedJobPage, error) {
	svc, err := s.failedJobs()
	if err != nil {
		return services.FailedJobPage{}, err
	}
	q := services.FailedJobQuery{Limit: req.Limit}
	if strings.TrimSpace(req.Type) != "" {
		if q.Type, err = parseFailedJobType(req.Type); err != nil {
			return services.FailedJobPage{}, err
		}
	}
	if tenant := strings.TrimSpace(req.Tenant); tenant != "" {
		t, err := s.GetTenant(ctx, tenant)
		if err != nil {
			return services.FailedJobPage{}, errxtrace.Wrap("failed to find tenant", err)
		}
		q.Scope.TenantID = t.ID
	}
	return svc.List(ctx, q)
}

// RetryFailedJob puts a failed job of any tenant back in its queue and
// writes an admin.failed_job_retry audit row, also when the retry is
// refused.
func (s *Service) RetryFailedJob(ctx context.Context, jobType, id string) (*models.FailedJob, error) {
	return s.actOnFailedJob(ctx, auditActionFailedJobRetry, jobType, id, (*services.FailedJobService).Retry)
}

// DiscardFailedJob marks a failed job of any tenant discarded and writes an
// admin.failed_job_discard audit row. The job keeps its row and its error.
func (s *Service) DiscardFailedJob(ctx context.Context, jobType, id string) (*models.FailedJob, error) {
	return s.actOnFailedJob(ctx, auditActionFailedJobDiscard, jobType, id, (*services.FailedJobService).Discard)
}

type failedJobAction func(*services.FailedJobService, context.Context, models.WorkerType, registry.FailedJobScope, string) (*models.FailedJob, error)

func (s *Service) actOnFailedJob(ctx context.Context, action, rawType, id string, do failedJobAction) (*models.FailedJob, error) {
	svc, err := s.failedJobs()
	if err != nil {
		return nil, err
	}
	jobType, err := parseFailedJobType(rawType)
	if err != nil {
		s.logFailedJobAction(ctx, action, rawType, id, nil, err)
		return nil, err
	}
	job, err := do(svc, ctx, jobType, registry.FailedJobScope{}, id)
	s.logFailedJobAction(ctx, action, string(jobType), id, job, err)
	return job, err
}

// logFailedJobAction writes the audit row of a CLI retry or discard. The
// actor is nil, as for every CLI action (see logAdminAction).
func (s *Service) logFailedJobAction(ctx context.Context, action, jobType, id string, job *models.FailedJob, opErr error) {
	if s.factorySet == nil || s.factorySet.AuditLogRegistry == nil {
		return
	}
	entry := models.AuditLog{
		Action:  action,
		Success: opErr == nil,
	}
	if jobType != "" {
		entry.EntityType = new(jobType)
	}
	if id != "" {
		entry.EntityID = new(id)
	}
	if job != nil && job.TenantID != "" {
		entry.TenantID = new(job.TenantID)
	}
	if opErr != nil {
		entry.ErrorMessage = new(opErr.Error())
	}
	if _, err := s.factorySet.AuditLogRegistry.Create(ctx, entry); err != nil {
		slog.Error("Failed to write failed-job audit log entry", "action", action, "job_type", jobType, "error", err)
	}
}
