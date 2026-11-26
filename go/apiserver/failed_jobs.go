package apiserver

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/jsonapi"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/services"
)

// Failed-job audit actions (#1307). The admin plane follows the
// "admin.<noun>_<verb>" pattern and services/admin writes the same
// literals for the CLI; the group plane writes the bare form, as the other
// group-admin actions do.
const (
	AuditActionAdminFailedJobList    = "admin.failed_job_list"
	AuditActionAdminFailedJobRetry   = "admin.failed_job_retry"
	AuditActionAdminFailedJobDiscard = "admin.failed_job_discard"
	AuditActionFailedJobRetry        = "failed_job_retry"
)

// JSON:API error codes of the failed-jobs endpoints.
const (
	// FailedJobUnknownTypeCode: the {jobType} segment or the type filter
	// is not a job type that keeps failed jobs. 404 on a path, 400 on a
	// filter.
	FailedJobUnknownTypeCode = "failed_job.unknown_type"
	// FailedJobNotFoundCode: no failed job with the id in the scope, also
	// one retried or discarded since the list was read. 404.
	FailedJobNotFoundCode = "failed_job.not_found"
	// FailedJobNotRetryableCode: the job's type can only be discarded. 409.
	FailedJobNotRetryableCode = "failed_job.not_retryable"
)

// failedJobResourceType is the JSON:API resource type of a failed job.
const failedJobResourceType = "failed_jobs"

// FailedJobView is the attributes block of a failed job in a group.
type FailedJobView struct {
	// JobType is the worker type that ran the job.
	JobType string `json:"job_type" example:"export"`
	// Title names the job: an export's description, a file's title.
	Title string `json:"title"`
	// Error is the last error the job recorded.
	Error    string `json:"error"`
	Attempts int    `json:"attempts" example:"3"`
	// Retryable is false for a type whose jobs can only be discarded.
	Retryable bool       `json:"retryable"`
	CreatedAt time.Time  `json:"created_at"`
	FailedAt  *time.Time `json:"failed_at,omitempty"`
	// UserID is the member who started the job.
	UserID string `json:"user_id,omitempty"`
}

// AdminFailedJobView is the attributes block of a failed job on the
// system-admin plane: the group view plus where the job belongs.
type AdminFailedJobView struct {
	JobType    string     `json:"job_type" example:"export"`
	Title      string     `json:"title"`
	Error      string     `json:"error"`
	Attempts   int        `json:"attempts" example:"3"`
	Retryable  bool       `json:"retryable"`
	CreatedAt  time.Time  `json:"created_at"`
	FailedAt   *time.Time `json:"failed_at,omitempty"`
	UserID     string     `json:"user_id,omitempty"`
	TenantID   string     `json:"tenant_id"`
	TenantName string     `json:"tenant_name,omitempty"`
	GroupID    string     `json:"group_id,omitempty"`
	GroupName  string     `json:"group_name,omitempty"`
	GroupSlug  string     `json:"group_slug,omitempty"`
}

// FailedJobResource is a failed job in a group. `id` is the job's id in its
// own table.
type FailedJobResource struct {
	Type       string        `json:"type" example:"failed_jobs"`
	ID         string        `json:"id"`
	Attributes FailedJobView `json:"attributes"`
}

// AdminFailedJobResource is a failed job on the system-admin plane.
type AdminFailedJobResource struct {
	Type       string             `json:"type" example:"failed_jobs"`
	ID         string             `json:"id"`
	Attributes AdminFailedJobView `json:"attributes"`
}

// FailedJobTypeInfo names a job type that keeps failed jobs and whether its
// jobs can be retried.
type FailedJobTypeInfo struct {
	Type      string `json:"type" example:"export"`
	Retryable bool   `json:"retryable"`
}

// FailedJobListMeta is the meta block of a failed-jobs list.
type FailedJobListMeta struct {
	Page       int                 `json:"page" example:"1"`
	PerPage    int                 `json:"per_page" example:"50"`
	Total      int                 `json:"total" example:"3"`
	TotalPages int                 `json:"total_pages" example:"1"`
	JobTypes   []FailedJobTypeInfo `json:"job_types"`
}

// FailedJobListEnvelope is the response of GET /g/{groupSlug}/failed-jobs.
type FailedJobListEnvelope struct {
	Data []FailedJobResource `json:"data"`
	Meta FailedJobListMeta   `json:"meta"`
}

// AdminFailedJobListEnvelope is the response of GET /admin/failed-jobs.
type AdminFailedJobListEnvelope struct {
	Data []AdminFailedJobResource `json:"data"`
	Meta FailedJobListMeta        `json:"meta"`
}

func failedJobTypeInfos() []FailedJobTypeInfo {
	types := models.FailedJobTypes()
	out := make([]FailedJobTypeInfo, 0, len(types))
	for _, t := range types {
		out = append(out, FailedJobTypeInfo{Type: string(t), Retryable: models.FailedJobRetryable(t)})
	}
	return out
}

func failedJobListMeta(page, perPage, total int) FailedJobListMeta {
	return FailedJobListMeta{
		Page:       page,
		PerPage:    perPage,
		Total:      total,
		TotalPages: jsonapi.ComputeTotalPages(total, perPage),
		JobTypes:   failedJobTypeInfos(),
	}
}

func failedJobView(job *models.FailedJob) FailedJobView {
	return FailedJobView{
		JobType:   string(job.Type),
		Title:     job.Title,
		Error:     job.Error,
		Attempts:  job.Attempts,
		Retryable: models.FailedJobRetryable(job.Type),
		CreatedAt: job.CreatedAt,
		FailedAt:  job.FailedAt,
		UserID:    job.UserID,
	}
}

// parseFailedJobTypeFilter reads the optional ?type filter. ok is false
// after it wrote a 400 for an unknown type.
func parseFailedJobTypeFilter(w http.ResponseWriter, r *http.Request) (models.WorkerType, bool) {
	raw := strings.TrimSpace(r.URL.Query().Get("type"))
	if raw == "" {
		return "", true
	}
	t := models.WorkerType(raw)
	if !models.IsFailedJobType(t) {
		_ = codedBadRequestError(w, r, errors.New("unknown job type"), FailedJobUnknownTypeCode)
		return "", false
	}
	return t, true
}

// resolveFailedJobPath reads the {jobType} and {jobID} path segments. ok is
// false after it wrote a 404 for an unknown type.
func resolveFailedJobPath(w http.ResponseWriter, r *http.Request) (models.WorkerType, string, bool) {
	t := models.WorkerType(strings.TrimSpace(chi.URLParam(r, "jobType")))
	if !models.IsFailedJobType(t) {
		_ = codedNotFoundError(w, r, errors.New("unknown job type"), FailedJobUnknownTypeCode)
		return "", "", false
	}
	return t, chi.URLParam(r, "jobID"), true
}

// renderFailedJobActionError maps an error of a retry or discard to its
// response. The 4xx bodies carry a fixed message, not the registry error
// with its attributes.
func renderFailedJobActionError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, registry.ErrNotFound):
		_ = codedNotFoundError(w, r, errors.New("failed job not found"), FailedJobNotFoundCode)
	case errors.Is(err, registry.ErrUnknownFailedJobType):
		_ = codedNotFoundError(w, r, errors.New("unknown job type"), FailedJobUnknownTypeCode)
	case errors.Is(err, registry.ErrFailedJobNotRetryable):
		_ = codedConflictError(w, r, errors.New("this kind of job cannot be retried"), FailedJobNotRetryableCode, nil)
	default:
		_ = internalServerError(w, r, err)
	}
}

func writeFailedJobEnvelope(w http.ResponseWriter, v any) error {
	w.Header().Set("Content-Type", "application/vnd.api+json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("failed jobs: failed to encode response", "error", err)
		return err
	}
	return nil
}

// adminFailedJobsAPI backs /admin/failed-jobs (#1307): every tenant's
// failed jobs for the back office, with retry and discard.
type adminFailedJobsAPI struct {
	factorySet   *registry.FactorySet
	service      *services.FailedJobService
	auditService services.AuditLogger
}

// listFailedJobs returns the failed jobs of every tenant, newest failure
// first.
//
// @Summary List failed background jobs (admin)
// @Description Returns the background jobs that failed for good and were neither
// @Description retried nor discarded, across every tenant, newest failure first
// @Description (#1307). Each job is read from its type's own table; `id` is the row
// @Description id there. ?type limits the list to one job type and ?tenantID to one
// @Description tenant. `meta.job_types` lists the job types with whether each can be
// @Description retried.
// @Description An unknown ?type returns 400 with `failed_job.unknown_type`.
// @Tags admin
// @Produce json-api
// @Param type query string false "Job type" Enums(export,import,restore,thumbnail,intake,currency-migration)
// @Param tenantID query string false "Tenant ID"
// @Param page query int false "Page number (default 1)"
// @Param per_page query int false "Items per page (default 50, max 100)"
// @Success 200 {object} AdminFailedJobListEnvelope "OK"
// @Failure 400 {object} jsonapi.Errors "Bad Request - unknown job type"
// @Failure 401 {object} jsonapi.Errors "Unauthorized - back-office authentication required"
// @Failure 403 {object} jsonapi.Errors "Account disabled"
// @Router /admin/failed-jobs [get]
func (api *adminFailedJobsAPI) listFailedJobs(w http.ResponseWriter, r *http.Request) {
	jobType, ok := parseFailedJobTypeFilter(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	page, perPage := parsePagination(q.Get("page"), q.Get("per_page"))
	// The tenant filter is `tenantID`: the admin subtree is exempt from
	// the check that refuses a tenant in the query, as for /admin/groups.
	tenantID := strings.TrimSpace(q.Get("tenantID"))

	result, err := api.service.List(r.Context(), services.FailedJobQuery{
		Type:   jobType,
		Scope:  registry.FailedJobScope{TenantID: tenantID},
		Offset: (page - 1) * perPage,
		Limit:  perPage,
	})
	if err != nil {
		api.audit(r, AuditActionAdminFailedJobList, tenantID, "", "", err)
		_ = internalServerError(w, r, err)
		return
	}

	names := newAdminFailedJobNames(api.factorySet)
	data := make([]AdminFailedJobResource, 0, len(result.Jobs))
	for _, job := range result.Jobs {
		data = append(data, AdminFailedJobResource{
			Type:       failedJobResourceType,
			ID:         job.ID,
			Attributes: names.view(r, job),
		})
	}

	setPaginationHeaders(w, page, perPage, result.Total)
	writeErr := writeFailedJobEnvelope(w, AdminFailedJobListEnvelope{
		Data: data,
		Meta: failedJobListMeta(page, perPage, result.Total),
	})
	api.audit(r, AuditActionAdminFailedJobList, tenantID, "", "", writeErr)
}

// retryFailedJob puts a failed job back in its queue.
//
// @Summary Retry a failed background job (admin)
// @Description Puts the failed job back in its queue the way its type runs it again
// @Description (#1307): an export or import runs from the start, a thumbnail gets a
// @Description fresh set of attempts, an intake scan is due again at once. A restore
// @Description and a currency migration cannot be retried (409
// @Description `failed_job.not_retryable`); discard them instead. 404
// @Description `failed_job.unknown_type` for an unknown type, 404
// @Description `failed_job.not_found` when the job is not failed or was already
// @Description retried or discarded.
// @Tags admin
// @Param jobType path string true "Job type" Enums(export,import,restore,thumbnail,intake,currency-migration)
// @Param jobID path string true "Job ID"
// @Success 204 "No Content"
// @Failure 401 {object} jsonapi.Errors "Unauthorized - back-office authentication required"
// @Failure 403 {object} jsonapi.Errors "Account disabled"
// @Failure 404 {object} jsonapi.Errors "Not Found - unknown type or no such failed job"
// @Failure 409 {object} jsonapi.Errors "Conflict - this type of job cannot be retried"
// @Router /admin/failed-jobs/{jobType}/{jobID}/retry [post]
func (api *adminFailedJobsAPI) retryFailedJob(w http.ResponseWriter, r *http.Request) {
	api.act(w, r, AuditActionAdminFailedJobRetry, api.service.Retry)
}

// discardFailedJob marks a failed job discarded.
//
// @Summary Discard a failed background job (admin)
// @Description Marks the failed job discarded (#1307). It leaves the failed list and
// @Description the depth gauge but keeps its row and its error until the normal
// @Description cleanup removes it. Every job type can be discarded. 404
// @Description `failed_job.unknown_type` for an unknown type, 404
// @Description `failed_job.not_found` when the job is not failed or was already
// @Description retried or discarded.
// @Tags admin
// @Param jobType path string true "Job type" Enums(export,import,restore,thumbnail,intake,currency-migration)
// @Param jobID path string true "Job ID"
// @Success 204 "No Content"
// @Failure 401 {object} jsonapi.Errors "Unauthorized - back-office authentication required"
// @Failure 403 {object} jsonapi.Errors "Account disabled"
// @Failure 404 {object} jsonapi.Errors "Not Found - unknown type or no such failed job"
// @Router /admin/failed-jobs/{jobType}/{jobID}/discard [post]
func (api *adminFailedJobsAPI) discardFailedJob(w http.ResponseWriter, r *http.Request) {
	api.act(w, r, AuditActionAdminFailedJobDiscard, api.service.Discard)
}

// failedJobAction is a FailedJobService retry or discard.
type failedJobAction func(ctx context.Context, jobType models.WorkerType, scope registry.FailedJobScope, id string) (*models.FailedJob, error)

// act runs a retry or discard across tenants and audits it, also when it
// fails, with the job's tenant when the job was found.
func (api *adminFailedJobsAPI) act(w http.ResponseWriter, r *http.Request, action string, do failedJobAction) {
	jobType, jobID, ok := resolveFailedJobPath(w, r)
	if !ok {
		return
	}
	job, err := do(r.Context(), jobType, registry.FailedJobScope{}, jobID)
	tenantID := ""
	if job != nil {
		tenantID = job.TenantID
	}
	if err != nil {
		api.audit(r, action, tenantID, string(jobType), jobID, err)
		renderFailedJobActionError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
	api.audit(r, action, tenantID, string(jobType), jobID, nil)
}

// audit writes an admin audit row. The subject is the job: its type and
// its id in the type's table.
func (api *adminFailedJobsAPI) audit(r *http.Request, action, tenantID, jobType, jobID string, opErr error) {
	if api.auditService == nil {
		return
	}
	api.auditService.LogAdmin(r.Context(), services.AdminEvent{
		Action:      action,
		ActorID:     actorIDFromRequest(r),
		TenantID:    nullableString(tenantID),
		SubjectType: nullableString(jobType),
		SubjectID:   nullableString(jobID),
		Success:     opErr == nil,
		Request:     r,
		ErrMsg:      strPtrFromErr(opErr),
	})
}

// adminFailedJobNames resolves the tenant and group names of the jobs on
// one page, reading each tenant and group once.
type adminFailedJobNames struct {
	factorySet *registry.FactorySet
	tenants    map[string]string
	groups     map[string]*models.LocationGroup
}

func newAdminFailedJobNames(fs *registry.FactorySet) *adminFailedJobNames {
	return &adminFailedJobNames{
		factorySet: fs,
		tenants:    map[string]string{},
		groups:     map[string]*models.LocationGroup{},
	}
}

func (n *adminFailedJobNames) view(r *http.Request, job *models.FailedJob) AdminFailedJobView {
	base := failedJobView(job)
	view := AdminFailedJobView{
		JobType:    base.JobType,
		Title:      base.Title,
		Error:      base.Error,
		Attempts:   base.Attempts,
		Retryable:  base.Retryable,
		CreatedAt:  base.CreatedAt,
		FailedAt:   base.FailedAt,
		UserID:     base.UserID,
		TenantID:   job.TenantID,
		TenantName: n.tenant(r, job.TenantID),
		GroupID:    job.GroupID,
	}
	if g := n.group(r, job.GroupID); g != nil {
		view.GroupName = g.Name
		view.GroupSlug = g.Slug
	}
	return view
}

// tenant returns the tenant's name, empty when it cannot be read: the list
// still renders with the id.
func (n *adminFailedJobNames) tenant(r *http.Request, id string) string {
	if id == "" {
		return ""
	}
	if name, ok := n.tenants[id]; ok {
		return name
	}
	name := ""
	if t, err := n.factorySet.TenantRegistry.Get(r.Context(), id); err == nil {
		name = t.Name
	} else {
		slog.Warn("admin failed jobs: failed to read tenant", "tenant_id", id, "error", err)
	}
	n.tenants[id] = name
	return name
}

func (n *adminFailedJobNames) group(r *http.Request, id string) *models.LocationGroup {
	if id == "" {
		return nil
	}
	if g, ok := n.groups[id]; ok {
		return g
	}
	var group *models.LocationGroup
	if item, err := n.factorySet.LocationGroupRegistry.GetAdmin(r.Context(), id); err == nil && item != nil {
		group = item.Group
	} else if err != nil {
		slog.Warn("admin failed jobs: failed to read group", "group_id", id, "error", err)
	}
	n.groups[id] = group
	return group
}

// groupFailedJobsAPI backs /g/{groupSlug}/failed-jobs (#1307): the group's
// own failed jobs for its admins, with retry.
type groupFailedJobsAPI struct {
	service      *services.FailedJobService
	auditService services.AuditLogger
}

// GroupFailedJobs mounts the group's failed jobs. The subtree is for group
// admins only. Every query carries the group of the route, so a group admin
// never reaches another group's jobs.
func GroupFailedJobs(params Params, groupService *services.GroupService, auditService services.AuditLogger) func(r chi.Router) {
	api := &groupFailedJobsAPI{
		service:      services.NewFailedJobService(params.FactorySet.FailedJobRegistry),
		auditService: auditService,
	}
	return func(r chi.Router) {
		r.Use(requireGroupAdmin(groupService))
		r.Get("/", api.list)
		r.Post("/{jobType}/{jobID}/retry", api.retry)
	}
}

func groupFailedJobScope(r *http.Request) (registry.FailedJobScope, bool) {
	group := groupFromContext(r.Context())
	if group == nil || group.ID == "" || group.TenantID == "" {
		return registry.FailedJobScope{}, false
	}
	return registry.FailedJobScope{TenantID: group.TenantID, GroupID: group.ID}, true
}

// list returns the group's failed jobs, newest failure first.
//
// @Summary List the group's failed background jobs
// @Description Returns the group's background jobs that failed for good and were
// @Description neither retried nor discarded, newest failure first (#1307). Group
// @Description admins only. ?type limits the list to one job type; an unknown type
// @Description returns 400 with `failed_job.unknown_type`. `meta.job_types` lists the
// @Description job types with whether each can be retried.
// @Tags failed-jobs
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param type query string false "Job type" Enums(export,import,restore,thumbnail,intake,currency-migration)
// @Param page query int false "Page number (default 1)"
// @Param per_page query int false "Items per page (default 50, max 100)"
// @Success 200 {object} FailedJobListEnvelope "OK"
// @Failure 400 {object} jsonapi.Errors "Bad Request - unknown job type"
// @Failure 401 {object} jsonapi.Errors "Unauthorized"
// @Failure 403 {object} jsonapi.Errors "Forbidden - group admins only"
// @Router /g/{groupSlug}/failed-jobs [get]
func (api *groupFailedJobsAPI) list(w http.ResponseWriter, r *http.Request) {
	scope, ok := groupFailedJobScope(r)
	if !ok {
		_ = internalServerError(w, r, errors.New("group context required"))
		return
	}
	jobType, ok := parseFailedJobTypeFilter(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	page, perPage := parsePagination(q.Get("page"), q.Get("per_page"))
	result, err := api.service.List(r.Context(), services.FailedJobQuery{
		Type:   jobType,
		Scope:  scope,
		Offset: (page - 1) * perPage,
		Limit:  perPage,
	})
	if err != nil {
		_ = internalServerError(w, r, err)
		return
	}
	data := make([]FailedJobResource, 0, len(result.Jobs))
	for _, job := range result.Jobs {
		data = append(data, FailedJobResource{Type: failedJobResourceType, ID: job.ID, Attributes: failedJobView(job)})
	}
	setPaginationHeaders(w, page, perPage, result.Total)
	_ = writeFailedJobEnvelope(w, FailedJobListEnvelope{Data: data, Meta: failedJobListMeta(page, perPage, result.Total)})
}

// retry puts one of the group's failed jobs back in its queue.
//
// @Summary Retry one of the group's failed background jobs
// @Description Puts the group's failed job back in its queue (#1307). Group admins
// @Description only. A restore and a currency migration cannot be retried (409
// @Description `failed_job.not_retryable`). 404 `failed_job.unknown_type` for an
// @Description unknown type, 404 `failed_job.not_found` when the group has no such
// @Description failed job, also one of another group.
// @Tags failed-jobs
// @Param groupSlug path string true "Group slug"
// @Param jobType path string true "Job type" Enums(export,import,restore,thumbnail,intake,currency-migration)
// @Param jobID path string true "Job ID"
// @Success 204 "No Content"
// @Failure 401 {object} jsonapi.Errors "Unauthorized"
// @Failure 403 {object} jsonapi.Errors "Forbidden - group admins only"
// @Failure 404 {object} jsonapi.Errors "Not Found - unknown type or no such failed job"
// @Failure 409 {object} jsonapi.Errors "Conflict - this type of job cannot be retried"
// @Router /g/{groupSlug}/failed-jobs/{jobType}/{jobID}/retry [post]
func (api *groupFailedJobsAPI) retry(w http.ResponseWriter, r *http.Request) {
	scope, ok := groupFailedJobScope(r)
	if !ok {
		_ = internalServerError(w, r, errors.New("group context required"))
		return
	}
	jobType, jobID, ok := resolveFailedJobPath(w, r)
	if !ok {
		return
	}
	_, err := api.service.Retry(r.Context(), jobType, scope, jobID)
	api.audit(r, scope, string(jobType), jobID, err)
	if err != nil {
		renderFailedJobActionError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (api *groupFailedJobsAPI) audit(r *http.Request, scope registry.FailedJobScope, jobType, jobID string, opErr error) {
	if api.auditService == nil {
		return
	}
	var userID *string
	if user := appctx.UserFromContext(r.Context()); user != nil {
		userID = nullableString(user.ID)
	}
	api.auditService.LogAuth(r.Context(), services.AuthEvent{
		Action:     AuditActionFailedJobRetry,
		UserID:     userID,
		TenantID:   nullableString(scope.TenantID),
		Success:    opErr == nil,
		Request:    r,
		ErrMsg:     strPtrFromErr(opErr),
		EntityType: nullableString(jobType),
		EntityID:   nullableString(jobID),
	})
}
