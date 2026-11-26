package models

import (
	"context"
	"crypto/sha256"
	"database/sql/driver"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"time"

	"github.com/jellydator/validation"

	"go.5x5.cz/inventario/models/rules"
)

var (
	_ validation.Validatable            = (*IntakeSession)(nil)
	_ validation.ValidatableWithContext = (*IntakeSession)(nil)
	_ TenantGroupAwareIDable            = (*IntakeSession)(nil)
	_ validation.Validatable            = (*IntakeSource)(nil)
	_ validation.ValidatableWithContext = (*IntakeSource)(nil)
	_ TenantGroupAwareIDable            = (*IntakeSource)(nil)
	_ validation.Validatable            = (*IntakeCandidate)(nil)
	_ validation.ValidatableWithContext = (*IntakeCandidate)(nil)
	_ TenantGroupAwareIDable            = (*IntakeCandidate)(nil)
)

// IntakeOrigin says how the files of an intake session arrived.
type IntakeOrigin string

const (
	// IntakeOriginUpload is a batch a member dropped in the app.
	IntakeOriginUpload IntakeOrigin = "upload"
	// IntakeOriginExternal is a document another system delivered, such as a
	// document archive's webhook (#2809). Its sources carry the external
	// identity.
	IntakeOriginExternal IntakeOrigin = "external"
)

// IntakeSourceStatus is where a source stands in the scan queue.
type IntakeSourceStatus string

const (
	// IntakeSourcePending waits for the background worker; NextAttemptAt
	// says when it may run.
	IntakeSourcePending IntakeSourceStatus = "pending"
	// IntakeSourceScanning is held by a worker that is calling the AI
	// provider.
	IntakeSourceScanning IntakeSourceStatus = "scanning"
	// IntakeSourceScanned has its candidates stored. It is not scanned again
	// unless a user asks for a rescan.
	IntakeSourceScanned IntakeSourceStatus = "scanned"
	// IntakeSourceFailed could not be scanned; ErrorCode says why. A user can
	// retry it.
	IntakeSourceFailed IntakeSourceStatus = "failed"
	// IntakeSourceCanceled was stopped by a user before it was scanned. A
	// resume puts it back in the queue.
	IntakeSourceCanceled IntakeSourceStatus = "canceled"
)

// IsQueued reports whether the source waits for or is in a scan.
func (s IntakeSourceStatus) IsQueued() bool {
	return s == IntakeSourcePending || s == IntakeSourceScanning
}

// IntakeCandidateOutcome is what became of a detected product.
type IntakeCandidateOutcome string

const (
	// IntakeCandidatePending has no draft yet.
	IntakeCandidatePending IntakeCandidateOutcome = "pending"
	// IntakeCandidateCreated has its draft; DraftID names it.
	IntakeCandidateCreated IntakeCandidateOutcome = "created"
	// IntakeCandidateSkipped was not kept: the user left it unselected when
	// the drafts were created.
	IntakeCandidateSkipped IntakeCandidateOutcome = "skipped"
	// IntakeCandidateFailed could not be made into a draft; ErrorCode says
	// why. Creating the drafts again retries it.
	IntakeCandidateFailed IntakeCandidateOutcome = "failed"
)

// Wait reasons of a pending source.
const (
	// IntakeWaitRateLimited: the author's hourly scan limit is reached; the
	// source runs when the limit allows it again.
	IntakeWaitRateLimited = "rate_limited"
	// IntakeWaitRetry: the provider failed in a way that may pass; the scan
	// is retried later.
	IntakeWaitRetry = "retry"
)

// Limits on the client-supplied values of an intake session.
const (
	// IntakeClientKeyMaxLength caps the idempotency key of a session.
	IntakeClientKeyMaxLength = 128
	// IntakeExternalValueMaxLength caps each part of a source's external
	// identity.
	IntakeExternalValueMaxLength = 255
	// IntakeCandidateQuantityMax caps the reviewed quantity of a product.
	IntakeCandidateQuantityMax = 100000
)

// IntakeFieldGuess is one value the AI scan read for a product, with the
// provider's confidence between 0 and 1. Value is a string, a number or a list
// of strings, as in the scan response.
type IntakeFieldGuess struct {
	Value      any     `json:"value"`
	Confidence float64 `json:"confidence"`
}

// IntakeFields maps a scan field name ("name", "original_price", ...) to what
// the scan read for it. It is stored as JSONB.
type IntakeFields map[string]IntakeFieldGuess

// Value implements driver.Valuer. A nil map is stored as an empty object.
func (f IntakeFields) Value() (driver.Value, error) {
	if f == nil {
		return []byte("{}"), nil
	}
	return json.Marshal(map[string]IntakeFieldGuess(f))
}

// Scan implements sql.Scanner. NULL and empty input give an empty map.
func (f *IntakeFields) Scan(value any) error {
	var raw []byte
	switch v := value.(type) {
	case nil:
		*f = IntakeFields{}
		return nil
	case []byte:
		raw = v
	case string:
		raw = []byte(v)
	default:
		return fmt.Errorf("cannot scan %T into IntakeFields", value)
	}
	if len(raw) == 0 {
		*f = IntakeFields{}
		return nil
	}
	out := IntakeFields{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return err
	}
	*f = out
	return nil
}

// IntakeWarning is a note the scan attached to its result, such as an
// unreadable serial number. Field is empty for a note about the whole source.
type IntakeWarning struct {
	Code   string `json:"code"`
	Field  string `json:"field,omitempty"`
	Detail string `json:"detail,omitempty"`
}

// IntakeSession is a batch of files a member dropped together, or a document
// another system delivered (#2810). The files form sources, one per purchase;
// the background worker scans each source once and stores the products it
// detected as candidates. The member reviews the candidates, keeps some, and
// gets exactly one draft (#2812) per kept candidate.
//
// A session belongs to its group like a draft: every member can see it, and
// members who can edit items can change it. ClientKey is the client's identity
// for the session: creating a session with a key the group already has returns
// that session, so a retry after a lost response does not queue the files
// twice.
//
//ptah:schema:rls:enable table="intake_sessions" comment="Enable RLS for multi-tenant intake session isolation"
//ptah:schema:rls:policy name="intake_session_isolation" table="intake_sessions" for="ALL" to="inventario_app" using="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != ''" with_check="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != ''" comment="Ensures intake sessions can only be accessed and modified by their tenant and group with required contexts"
//ptah:schema:rls:policy name="intake_session_background_worker_access" table="intake_sessions" for="ALL" to="inventario_background_worker" using="true" with_check="true" comment="Allows background workers to read and purge intake sessions across all groups"
//ptah:schema:table name="intake_sessions"
type IntakeSession struct {
	//ptah:embedded mode="inline"
	TenantGroupAwareEntityID

	// ClientKey identifies the session for the client that created it;
	// unique in the group.
	//ptah:schema:field name="client_key" type="TEXT" not_null="true"
	ClientKey string `json:"client_key" db:"client_key"`

	// Origin says how the files arrived.
	//ptah:schema:field name="origin" type="TEXT" not_null="true" default="upload"
	Origin IntakeOrigin `json:"origin" db:"origin" userinput:"false"`

	//ptah:schema:field name="created_at" type="TIMESTAMP" not_null="true" default_expr="CURRENT_TIMESTAMP"
	CreatedAt time.Time `json:"created_at" db:"created_at" userinput:"false"`

	//ptah:schema:field name="updated_at" type="TIMESTAMP" not_null="true" default_expr="CURRENT_TIMESTAMP"
	UpdatedAt time.Time `json:"updated_at" db:"updated_at" userinput:"false"`
}

// IntakeSessionIndexes defines the postgres indexes for intake_sessions.
type IntakeSessionIndexes struct {
	// Unique index for the immutable UUID.
	//ptah:schema:index name="idx_intake_sessions_uuid" fields="uuid" unique="true" table="intake_sessions"
	_ int

	// One session per client key in a group: the idempotency of create.
	//ptah:schema:index name="idx_intake_sessions_group_client_key" fields="group_id,client_key" unique="true" table="intake_sessions"
	_ int

	// Composite index for tenant+group RLS-filtered queries and the purges.
	//ptah:schema:index name="idx_intake_sessions_tenant_group" fields="tenant_id,group_id" table="intake_sessions"
	_ int

	// The sessions list reads a group's sessions, newest first.
	//ptah:schema:index name="idx_intake_sessions_group_created" fields="group_id,created_at" table="intake_sessions"
	_ int
}

func (*IntakeSession) Validate() error {
	return ErrMustUseValidateWithContext
}

// ValidateWithContext checks a stored session.
func (s *IntakeSession) ValidateWithContext(ctx context.Context) error {
	return validation.ValidateStructWithContext(ctx, s,
		validation.Field(&s.TenantGroupAwareEntityID),
		validation.Field(&s.ClientKey, rules.NotEmpty, validation.Length(1, IntakeClientKeyMaxLength), clientKeyRule),
		validation.Field(&s.Origin, validation.Required, validation.In(IntakeOriginUpload, IntakeOriginExternal)),
	)
}

// IntakeSource is one purchase within a session: the files the AI scan reads
// together in one call. By default every dropped file is a source of its own.
// Files the member said show one item form one source with OneItem set, and
// the scan then keeps a single product for it, so several photos of one thing
// are not taken for a receipt that lists several products.
//
// The files are linked to the source through file_links with entity type
// "intake", which keeps them while the session needs them.
//
// ExternalSystem, ExternalInstance and ExternalDocumentID identify a document
// delivered by another system (#2809), for instance "paperless-ngx", the
// archive's base URL and its document id. Together with the group they are
// unique, so a replayed delivery finds the source it created before. A source
// made from an upload leaves them empty.
//
//ptah:schema:rls:enable table="intake_sources" comment="Enable RLS for multi-tenant intake source isolation"
//ptah:schema:rls:policy name="intake_source_isolation" table="intake_sources" for="ALL" to="inventario_app" using="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != ''" with_check="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != ''" comment="Ensures intake sources can only be accessed and modified by their tenant and group with required contexts"
//ptah:schema:rls:policy name="intake_source_background_worker_access" table="intake_sources" for="ALL" to="inventario_background_worker" using="true" with_check="true" comment="Allows the intake worker to claim and update intake sources across all groups"
//ptah:schema:table name="intake_sources"
type IntakeSource struct {
	//ptah:embedded mode="inline"
	TenantGroupAwareEntityID

	//ptah:schema:field name="session_id" type="TEXT" not_null="true" foreign="intake_sessions(id)" foreign_key_name="fk_intake_source_session" on_delete="CASCADE"
	SessionID string `json:"session_id" db:"session_id" userinput:"false"`

	// SortOrder orders the sources of a session as they were dropped.
	//ptah:schema:field name="sort_order" type="INTEGER" not_null="true" default="0"
	SortOrder int `json:"sort_order" db:"sort_order" userinput:"false"`

	// OneItem says that the files show one item, so the scan keeps one
	// product.
	//ptah:schema:field name="one_item" type="BOOLEAN" not_null="true" default="false"
	OneItem bool `json:"one_item" db:"one_item"`

	//ptah:schema:field name="status" type="TEXT" not_null="true" default="pending"
	Status IntakeSourceStatus `json:"status" db:"status" userinput:"false"`

	// Attempts counts the scans started for the source. It never goes
	// down, so it also tells the result of the current scan from the late
	// result of an earlier one.
	//ptah:schema:field name="attempts" type="INTEGER" not_null="true" default="0"
	Attempts int `json:"attempts" db:"attempts" userinput:"false"`

	// RetryBase is the part of Attempts that does not count against the
	// worker's limit of scans: a resume or a rescan sets it to Attempts, and
	// a wait for the rate limit raises it by one, since that scan never
	// reached the provider. See CountedAttempts.
	//ptah:schema:field name="retry_base" type="INTEGER" not_null="true" default="0"
	RetryBase int `json:"-" db:"retry_base" userinput:"false"`

	// NextAttemptAt is the earliest time the worker may scan a pending
	// source.
	//ptah:schema:field name="next_attempt_at" type="TIMESTAMP" not_null="true" default_expr="CURRENT_TIMESTAMP"
	NextAttemptAt time.Time `json:"next_attempt_at" db:"next_attempt_at" userinput:"false"`

	// ClaimedAt is when the worker took a scanning source.
	//ptah:schema:field name="claimed_at" type="TIMESTAMP"
	ClaimedAt *time.Time `json:"claimed_at,omitempty" db:"claimed_at" userinput:"false"`

	// WaitReason says why a pending source waits (IntakeWait*), empty when
	// it simply has not had its turn.
	//ptah:schema:field name="wait_reason" type="TEXT" not_null="true" default=""
	WaitReason string `json:"wait_reason" db:"wait_reason" userinput:"false"`

	// ErrorCode says why a failed source could not be scanned.
	//ptah:schema:field name="error_code" type="TEXT" not_null="true" default=""
	ErrorCode string `json:"error_code" db:"error_code" userinput:"false"`

	// Warnings are the notes of the stored scan.
	//ptah:schema:field name="warnings" type="JSONB"
	Warnings ValuerSlice[IntakeWarning] `json:"warnings" db:"warnings" userinput:"false"`

	//ptah:schema:field name="scanned_at" type="TIMESTAMP"
	ScannedAt *time.Time `json:"scanned_at,omitempty" db:"scanned_at" userinput:"false"`

	//ptah:schema:field name="external_system" type="TEXT" not_null="true" default=""
	ExternalSystem string `json:"external_system,omitempty" db:"external_system"`

	//ptah:schema:field name="external_instance" type="TEXT" not_null="true" default=""
	ExternalInstance string `json:"external_instance,omitempty" db:"external_instance"`

	//ptah:schema:field name="external_document_id" type="TEXT" not_null="true" default=""
	ExternalDocumentID string `json:"external_document_id,omitempty" db:"external_document_id"`

	//ptah:schema:field name="created_at" type="TIMESTAMP" not_null="true" default_expr="CURRENT_TIMESTAMP"
	CreatedAt time.Time `json:"created_at" db:"created_at" userinput:"false"`

	//ptah:schema:field name="updated_at" type="TIMESTAMP" not_null="true" default_expr="CURRENT_TIMESTAMP"
	UpdatedAt time.Time `json:"updated_at" db:"updated_at" userinput:"false"`
}

// IntakeSourceIndexes defines the postgres indexes for intake_sources.
type IntakeSourceIndexes struct {
	// Unique index for the immutable UUID.
	//ptah:schema:index name="idx_intake_sources_uuid" fields="uuid" unique="true" table="intake_sources"
	_ int

	// A session reads its sources in order.
	//ptah:schema:index name="idx_intake_sources_session_order" fields="session_id,sort_order" table="intake_sources"
	_ int

	// The worker claims the next due source across all groups.
	//ptah:schema:index name="idx_intake_sources_queue" fields="status,next_attempt_at" table="intake_sources" condition="status IN ('pending', 'scanning')"
	_ int

	// A document another system delivered is one source per group: a replay
	// finds the source it created (#2809).
	//ptah:schema:index name="idx_intake_sources_external" fields="group_id,external_system,external_instance,external_document_id" unique="true" table="intake_sources" condition="external_document_id <> ''"
	_ int

	// Composite index for tenant+group RLS-filtered queries and the purges.
	//ptah:schema:index name="idx_intake_sources_tenant_group" fields="tenant_id,group_id" table="intake_sources"
	_ int
}

// HasExternalIdentity reports whether another system delivered the source.
func (s *IntakeSource) HasExternalIdentity() bool {
	return s.ExternalDocumentID != ""
}

// CountedAttempts is the number of scans that count against the worker's
// limit since the source was last queued by a member.
func (s *IntakeSource) CountedAttempts() int {
	return s.Attempts - s.RetryBase
}

// InScan reports whether a worker holds a claim on the source: it is being
// scanned, or it was canceled while the scan was running.
func (s *IntakeSource) InScan() bool {
	return s.Status == IntakeSourceScanning || (s.Status == IntakeSourceCanceled && s.ClaimedAt != nil)
}

func (*IntakeSource) Validate() error {
	return ErrMustUseValidateWithContext
}

// ValidateWithContext checks a stored source.
func (s *IntakeSource) ValidateWithContext(ctx context.Context) error {
	return validation.ValidateStructWithContext(ctx, s,
		validation.Field(&s.TenantGroupAwareEntityID),
		validation.Field(&s.SessionID, rules.NotEmpty),
		validation.Field(&s.Status, validation.Required, validation.In(
			IntakeSourcePending, IntakeSourceScanning, IntakeSourceScanned, IntakeSourceFailed, IntakeSourceCanceled,
		)),
		validation.Field(&s.Attempts, validation.Min(0)),
		validation.Field(&s.RetryBase, validation.Min(0), validation.Max(s.Attempts)),
		validation.Field(&s.ExternalSystem, validation.Length(0, IntakeExternalValueMaxLength),
			validation.When(s.ExternalDocumentID != "", validation.Required)),
		validation.Field(&s.ExternalInstance, validation.Length(0, IntakeExternalValueMaxLength)),
		validation.Field(&s.ExternalDocumentID, validation.Length(0, IntakeExternalValueMaxLength)),
	)
}

// IntakeCandidate is one product the scan detected in a source. Its id is its
// identity for the rest of the session: the draft made for it carries the
// client key "intake:<candidate id>", so a retry, a second device or a lost
// response finds the draft instead of making another one.
//
// Fields is what the scan read, with the confidence of each value. Selected,
// AcceptedFields and Quantity are the member's review: whether to keep the
// product, which of the read values go into the draft, and how many identical
// units it covers. A quantity above one stays one item with that count, as an
// item's count always means a bundle of identical units.
//
//ptah:schema:rls:enable table="intake_candidates" comment="Enable RLS for multi-tenant intake candidate isolation"
//ptah:schema:rls:policy name="intake_candidate_isolation" table="intake_candidates" for="ALL" to="inventario_app" using="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != ''" with_check="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != ''" comment="Ensures intake candidates can only be accessed and modified by their tenant and group with required contexts"
//ptah:schema:rls:policy name="intake_candidate_background_worker_access" table="intake_candidates" for="ALL" to="inventario_background_worker" using="true" with_check="true" comment="Allows the intake worker to store detected products across all groups"
//ptah:schema:table name="intake_candidates"
type IntakeCandidate struct {
	//ptah:embedded mode="inline"
	TenantGroupAwareEntityID

	//ptah:schema:field name="session_id" type="TEXT" not_null="true" foreign="intake_sessions(id)" foreign_key_name="fk_intake_candidate_session" on_delete="CASCADE"
	SessionID string `json:"session_id" db:"session_id" userinput:"false"`

	//ptah:schema:field name="source_id" type="TEXT" not_null="true" foreign="intake_sources(id)" foreign_key_name="fk_intake_candidate_source" on_delete="CASCADE"
	SourceID string `json:"source_id" db:"source_id" userinput:"false"`

	// SortOrder orders the candidates of a source as the scan listed them,
	// most prominent first.
	//ptah:schema:field name="sort_order" type="INTEGER" not_null="true" default="0"
	SortOrder int `json:"sort_order" db:"sort_order" userinput:"false"`

	//ptah:schema:field name="fields" type="JSONB" not_null="true"
	Fields IntakeFields `json:"fields" db:"fields" userinput:"false"`

	//ptah:schema:field name="selected" type="BOOLEAN" not_null="true" default="true"
	Selected bool `json:"selected" db:"selected"`

	//ptah:schema:field name="accepted_fields" type="JSONB"
	AcceptedFields ValuerSlice[string] `json:"accepted_fields" db:"accepted_fields"`

	//ptah:schema:field name="quantity" type="INTEGER" not_null="true" default="1"
	Quantity int `json:"quantity" db:"quantity"`

	//ptah:schema:field name="outcome" type="TEXT" not_null="true" default="pending"
	Outcome IntakeCandidateOutcome `json:"outcome" db:"outcome" userinput:"false"`

	// DraftID is the draft made for the candidate. The draft can be
	// discarded or promoted later, so there is no foreign key.
	//ptah:schema:field name="draft_id" type="TEXT"
	DraftID *string `json:"draft_id,omitempty" db:"draft_id" userinput:"false"`

	// ErrorCode says why the draft could not be made.
	//ptah:schema:field name="error_code" type="TEXT" not_null="true" default=""
	ErrorCode string `json:"error_code" db:"error_code" userinput:"false"`

	//ptah:schema:field name="created_at" type="TIMESTAMP" not_null="true" default_expr="CURRENT_TIMESTAMP"
	CreatedAt time.Time `json:"created_at" db:"created_at" userinput:"false"`

	//ptah:schema:field name="updated_at" type="TIMESTAMP" not_null="true" default_expr="CURRENT_TIMESTAMP"
	UpdatedAt time.Time `json:"updated_at" db:"updated_at" userinput:"false"`
}

// IntakeCandidateIndexes defines the postgres indexes for intake_candidates.
type IntakeCandidateIndexes struct {
	// Unique index for the immutable UUID.
	//ptah:schema:index name="idx_intake_candidates_uuid" fields="uuid" unique="true" table="intake_candidates"
	_ int

	// A session reads all its candidates at once.
	//ptah:schema:index name="idx_intake_candidates_session" fields="session_id" table="intake_candidates"
	_ int

	// A source's candidates in order; a rescan replaces them.
	//ptah:schema:index name="idx_intake_candidates_source_order" fields="source_id,sort_order" table="intake_candidates"
	_ int

	// Composite index for tenant+group RLS-filtered queries and the purges.
	//ptah:schema:index name="idx_intake_candidates_tenant_group" fields="tenant_id,group_id" table="intake_candidates"
	_ int
}

// DraftClientKey is the client key of the draft made for the candidate.
func (c *IntakeCandidate) DraftClientKey() string {
	return IntakeDraftClientKey(c.ID)
}

// IntakeDraftClientKey is the client key of the draft made for the candidate
// with the given id.
func IntakeDraftClientKey(candidateID string) string {
	return "intake:" + candidateID
}

// IntakeDeliveredDraftClientKey is the client key of the draft made for a
// document another system delivered (#2809), from the document's identity.
// The product of a delivery is the only one of its source, and the key does
// not change when the session is deleted and made again, so a repeat of the
// delivery finds the draft without the session.
func IntakeDeliveredDraftClientKey(system, instance, documentID string) string {
	sum := sha256.Sum256([]byte(system + "\x00" + instance + "\x00" + documentID))
	return "intake-doc:" + hex.EncodeToString(sum[:])
}

// DraftClientKey is the client key of the draft made for a product of the
// source: one key per delivered document, one per product otherwise.
func (s *IntakeSource) DraftClientKey(cand *IntakeCandidate) string {
	if s.HasExternalIdentity() {
		return IntakeDeliveredDraftClientKey(s.ExternalSystem, s.ExternalInstance, s.ExternalDocumentID)
	}
	return cand.DraftClientKey()
}

// IsUnfinished reports whether creating the drafts would still act on the
// candidate: it is kept and has no draft yet, or its draft failed.
func (c *IntakeCandidate) IsUnfinished() bool {
	return c.Selected && (c.Outcome == IntakeCandidatePending || c.Outcome == IntakeCandidateFailed)
}

// Accepts reports whether the member accepts the scanned value of field.
func (c *IntakeCandidate) Accepts(field string) bool {
	return slices.Contains(c.AcceptedFields, field)
}

func (*IntakeCandidate) Validate() error {
	return ErrMustUseValidateWithContext
}

// ValidateWithContext checks a stored candidate.
func (c *IntakeCandidate) ValidateWithContext(ctx context.Context) error {
	return validation.ValidateStructWithContext(ctx, c,
		validation.Field(&c.TenantGroupAwareEntityID),
		validation.Field(&c.SessionID, rules.NotEmpty),
		validation.Field(&c.SourceID, rules.NotEmpty),
		validation.Field(&c.Quantity, validation.Required, validation.Min(1), validation.Max(IntakeCandidateQuantityMax)),
		validation.Field(&c.Outcome, validation.Required, validation.In(
			IntakeCandidatePending, IntakeCandidateCreated, IntakeCandidateSkipped, IntakeCandidateFailed,
		)),
	)
}

// IntakeSessionState is the stage a session is in, derived from its sources
// and candidates.
type IntakeSessionState string

const (
	// IntakeSessionProcessing has sources waiting for or in a scan.
	IntakeSessionProcessing IntakeSessionState = "processing"
	// IntakeSessionReview has scanned products nobody decided on yet, or
	// kept products whose draft failed.
	IntakeSessionReview IntakeSessionState = "review"
	// IntakeSessionDone has nothing left to do: every kept product has its
	// draft. Failed or canceled sources may remain; the summary counts them.
	IntakeSessionDone IntakeSessionState = "done"
)

// IntakeSessionSummary counts the sources and candidates of a session.
type IntakeSessionSummary struct {
	Sources     int `json:"sources"`
	Files       int `json:"files"`
	Pending     int `json:"pending"`
	Scanning    int `json:"scanning"`
	Scanned     int `json:"scanned"`
	Failed      int `json:"failed"`
	Canceled    int `json:"canceled"`
	RateLimited int `json:"rate_limited"`

	Candidates int `json:"candidates"`
	Selected   int `json:"selected"`
	// Undecided counts the products with no outcome yet, kept or not.
	Undecided int `json:"undecided"`
	// Unfinished counts the kept products creating the drafts acts on:
	// no draft yet, or a draft that failed.
	Unfinished       int `json:"unfinished"`
	DraftsCreated    int `json:"drafts_created"`
	Skipped          int `json:"skipped"`
	CandidatesFailed int `json:"candidates_failed"`
}

// AddSource counts one source and its live files.
func (s *IntakeSessionSummary) AddSource(src *IntakeSource, files int) {
	s.Sources++
	s.Files += files
	switch src.Status {
	case IntakeSourcePending:
		s.Pending++
		if src.WaitReason == IntakeWaitRateLimited {
			s.RateLimited++
		}
	case IntakeSourceScanning:
		s.Scanning++
	case IntakeSourceScanned:
		s.Scanned++
	case IntakeSourceFailed:
		s.Failed++
	case IntakeSourceCanceled:
		s.Canceled++
	}
}

// AddCandidate counts one candidate.
func (s *IntakeSessionSummary) AddCandidate(c *IntakeCandidate) {
	s.Candidates++
	if c.Selected {
		s.Selected++
	}
	if c.IsUnfinished() {
		s.Unfinished++
	}
	if c.Outcome == IntakeCandidatePending {
		s.Undecided++
	}
	switch c.Outcome {
	case IntakeCandidateCreated:
		s.DraftsCreated++
	case IntakeCandidateSkipped:
		s.Skipped++
	case IntakeCandidateFailed:
		s.CandidatesFailed++
	}
}

// State derives the stage of the session from the counts.
func (s IntakeSessionSummary) State() IntakeSessionState {
	switch {
	case s.Pending+s.Scanning > 0:
		return IntakeSessionProcessing
	case s.Undecided+s.Unfinished > 0:
		return IntakeSessionReview
	default:
		return IntakeSessionDone
	}
}

// Open reports whether the session is unfinished work, which the inbox
// lists: a file waits for its scan or is being scanned, a product waits for
// a decision or for its draft, or a file could not be scanned. A session
// whose files all failed stays open until a member retries or removes it.
func (s IntakeSessionSummary) Open() bool {
	return s.State() != IntakeSessionDone || s.Failed > 0
}
