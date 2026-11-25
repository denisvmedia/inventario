package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/jellydator/validation"
	"github.com/shopspring/decimal"

	"go.5x5.cz/inventario/internal/aivision"
	"go.5x5.cz/inventario/internal/mimekit"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

// ErrIntakeDocumentChanged is returned when another system delivers a
// document the group already has with different values or a different file.
// The first delivery stands: the draft or item it made is not changed, and no
// second draft is made.
var ErrIntakeDocumentChanged = errx.NewSentinel("the document was delivered before with different values")

// intakeDocumentTitleMaxRunes is the draft's limit on a title.
const intakeDocumentTitleMaxRunes = 255

// intakeSessionClientKeyPrefix starts the client key of the intake session a
// delivery makes; the document id follows.
const intakeSessionClientKeyPrefix = "external:"

// IntakeDelivery is a document another system sends to the intake endpoint
// (#2809), as the raw strings of the request. Empty means absent.
type IntakeDelivery struct {
	// Title becomes the draft's name.
	Title string
	// SourceSystem, SourceInstance and DocumentID identify the document:
	// the system, its installation and the document's id there.
	SourceSystem   string
	SourceInstance string
	DocumentID     string
	// SourceURL links to the document in the other system. It is stored and
	// shown, never fetched.
	SourceURL string
	// SourceName is how the other system is shown, e.g. "Paperless-ngx".
	SourceName string
	// Seller, PurchaseDate, Amount and Currency are the purchase data the
	// other system knows. What it does not send stays unknown.
	Seller       string
	PurchaseDate string
	Amount       string
	Currency     string
}

// IntakeDeliveredFile is the file of a delivery, already stored as a blob by
// the caller.
type IntakeDeliveredFile struct {
	BlobKey  string
	Name     string
	MIMEType string
	Size     int64
	SHA256   string
}

// IntakeDeliveryResult is the outcome of IntakeService.Deliver.
type IntakeDeliveryResult struct {
	Document *models.IntakeDocument
	State    models.IntakeDocumentState
	// Created reports that this call stored the document; a replay finds it.
	Created bool
	// File is the file of the group the delivered blob became, nil when the
	// blob was not used. The caller deletes a blob that was not used.
	File *models.FileEntity
}

// normalizedDelivery is a delivery checked and put in canonical form. Its
// JSON encoding, with the file's hash, is the payload fingerprint.
type normalizedDelivery struct {
	Title          string `json:"title"`
	SourceSystem   string `json:"source_system"`
	SourceInstance string `json:"source_instance"`
	DocumentID     string `json:"document_id"`
	SourceURL      string `json:"source_url"`
	SourceName     string `json:"source_name"`
	Seller         string `json:"seller"`
	PurchaseDate   string `json:"purchase_date"`
	Amount         string `json:"amount"`
	Currency       string `json:"currency"`
	FileSHA256     string `json:"file_sha256"`
}

// intakeSourceSystemPattern is the form of a source system name once
// lowercased: letters, digits, dots, dashes and underscores.
var intakeSourceSystemPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]*$`)

// intakeAmountPattern is an amount with an optional ISO currency code in
// front, the form Paperless-ngx gives a monetary field ("EUR12.30").
var intakeAmountPattern = regexp.MustCompile(`^([A-Za-z]{3})?(\d+(?:\.\d{1,2})?)$`)

// normalizeIntakeDelivery checks the delivery and puts it in canonical form.
// A malformed value is refused rather than dropped, so the other system sees
// the mistake in its log.
func normalizeIntakeDelivery(in IntakeDelivery, file *IntakeDeliveredFile) (normalizedDelivery, error) {
	n := normalizedDelivery{
		Title:          strings.TrimSpace(in.Title),
		SourceSystem:   strings.ToLower(strings.TrimSpace(in.SourceSystem)),
		SourceInstance: strings.TrimSpace(in.SourceInstance),
		DocumentID:     strings.TrimSpace(in.DocumentID),
		SourceURL:      strings.TrimSpace(in.SourceURL),
		SourceName:     strings.TrimSpace(in.SourceName),
		Seller:         strings.Join(strings.Fields(in.Seller), " "),
		Currency:       strings.ToUpper(strings.TrimSpace(in.Currency)),
	}
	if file != nil {
		n.FileSHA256 = file.SHA256
	}
	errs := validation.Errors{}
	if err := validation.Validate(n.Title, validation.Required, validation.RuneLength(1, intakeDocumentTitleMaxRunes)); err != nil {
		errs["title"] = err
	}
	if err := validation.Validate(n.SourceSystem, validation.Required, validation.Length(1, models.IntakeExternalValueMaxLength),
		validation.Match(intakeSourceSystemPattern).Error("must be letters, digits, dots, dashes or underscores")); err != nil {
		errs["source_system"] = err
	}
	if err := validation.Validate(n.SourceInstance, validation.Length(0, models.IntakeExternalValueMaxLength)); err != nil {
		errs["source_instance"] = err
	}
	if err := validation.Validate(n.DocumentID, validation.Required, validation.Length(1, models.IntakeExternalValueMaxLength)); err != nil {
		errs["document_id"] = err
	}
	if err := validation.Validate(n.SourceURL, validation.Length(0, models.IntakeSourceURLMaxLength), models.IntakeSourceURLRule); err != nil {
		errs["source_url"] = err
	}
	if err := validation.Validate(n.SourceName, validation.RuneLength(0, models.IntakeSourceNameMaxRunes)); err != nil {
		errs["source_name"] = err
	}
	if utf8.RuneCountInString(n.Seller) > models.MaxSellerNameRunes {
		errs["seller"] = validation.NewError("validation_intake_seller_length", "the seller must be at most {{.max}} characters").
			SetParams(map[string]any{"max": models.MaxSellerNameRunes})
	}
	if date, err := normalizeIntakeDate(in.PurchaseDate); err != nil {
		errs["purchase_date"] = err
	} else {
		n.PurchaseDate = date
	}
	if n.Currency != "" && !models.Currency(n.Currency).IsValid() {
		errs["currency"] = validation.NewError("validation_intake_currency", "unknown currency code")
	}
	if amount, code, err := normalizeIntakeAmount(in.Amount); err != nil {
		errs["amount"] = err
	} else {
		n.Amount = amount
		switch {
		case code == "":
		case n.Currency == "":
			n.Currency = code
		case n.Currency != code:
			errs["currency"] = validation.NewError("validation_intake_currency_mismatch", "the currency differs from the one in the amount")
		}
	}
	if len(errs) > 0 {
		return normalizedDelivery{}, errs
	}
	return n, nil
}

// normalizeIntakeDate accepts a date as YYYY-MM-DD or as the date part of an
// RFC 3339 time, and returns it as YYYY-MM-DD.
func normalizeIntakeDate(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", nil
	}
	if d, err := time.Parse(time.DateOnly, s); err == nil {
		return d.Format(time.DateOnly), nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.Format(time.DateOnly), nil
	}
	return "", validation.NewError("validation_intake_date", "must be a date as YYYY-MM-DD")
}

// normalizeIntakeAmount reads a non-negative amount with at most two
// decimals and an optional currency code in front. It returns the amount
// with two decimals and the code in upper case.
func normalizeIntakeAmount(raw string) (amount, code string, err error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", "", nil
	}
	invalid := validation.NewError("validation_intake_amount", "must be a non-negative number with at most two decimals, optionally after a currency code")
	m := intakeAmountPattern.FindStringSubmatch(s)
	if m == nil {
		return "", "", invalid
	}
	code = strings.ToUpper(m[1])
	if code != "" && !models.Currency(code).IsValid() {
		return "", "", invalid
	}
	d, err := decimal.NewFromString(m[2])
	if err != nil {
		return "", "", invalid
	}
	return d.StringFixed(2), code, nil
}

// fingerprint is the hash of the canonical delivery.
func (n normalizedDelivery) fingerprint() string {
	raw, _ := json.Marshal(n)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

// candidate is the product the delivery describes, with every delivered
// value accepted: the other system knows them, no scan guessed them.
func (n normalizedDelivery) candidate() (models.IntakeCandidate, error) {
	fields := models.IntakeFields{
		aivision.FieldNameName: {Value: n.Title, Confidence: 1},
	}
	if n.SourceURL != "" {
		fields[aivision.FieldNameURLs] = models.IntakeFieldGuess{Value: []any{n.SourceURL}, Confidence: 1}
	}
	if n.Seller != "" {
		fields[aivision.FieldNameSeller] = models.IntakeFieldGuess{Value: n.Seller, Confidence: 1}
	}
	if n.PurchaseDate != "" {
		fields[aivision.FieldNamePurchaseDate] = models.IntakeFieldGuess{Value: n.PurchaseDate, Confidence: 1}
	}
	if n.Amount != "" {
		amount, err := decimal.NewFromString(n.Amount)
		if err != nil {
			return models.IntakeCandidate{}, err
		}
		fields[aivision.FieldNameOriginalPrice] = models.IntakeFieldGuess{Value: amount.InexactFloat64(), Confidence: 1}
	}
	if n.Currency != "" {
		fields[aivision.FieldNameOriginalPriceCurrency] = models.IntakeFieldGuess{Value: n.Currency, Confidence: 1}
	}
	accepted := []string{}
	for _, name := range IntakeReviewFields {
		if _, ok := fields[name]; ok {
			accepted = append(accepted, name)
		}
	}
	return models.IntakeCandidate{Fields: fields, Selected: true, AcceptedFields: accepted, Quantity: 1}, nil
}

// Deliver turns a document another system delivered into a draft in the
// group's inbox (#2809). No AI scan runs: the values come from the other
// system, and what it does not send stays unknown. The file, when there is
// one, is stored as a file of the group and attached to the draft.
//
// The document's identity is the source system, its instance and the
// document id, unique in the group. A repeated delivery with the same values
// and file is a replay: it returns the document and makes nothing. A delivery
// with other values gets ErrIntakeDocumentChanged and changes nothing. The
// document record outlives the intake session and the draft, so a replay
// after the member deleted the session, discarded the draft or finished the
// item still makes nothing.
//
// Every step can be repeated: the file, the session and the draft are each
// recorded on the document once made, so a replay after a failure half way
// finishes the work without doing any of it twice.
func (s *IntakeService) Deliver(ctx context.Context, in IntakeDelivery, file *IntakeDeliveredFile) (*IntakeDeliveryResult, error) {
	n, err := normalizeIntakeDelivery(in, file)
	if err != nil {
		return nil, err
	}
	set, err := s.factorySet.CreateUserRegistrySet(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to create registry set", err)
	}
	docs := set.IntakeDocumentRegistry
	doc, created, err := docs.CreateOrGet(ctx, models.IntakeDocument{
		ExternalSystem:     n.SourceSystem,
		ExternalInstance:   n.SourceInstance,
		ExternalDocumentID: n.DocumentID,
		SourceName:         n.SourceName,
		SourceURL:          n.SourceURL,
		PayloadHash:        n.fingerprint(),
	})
	if err != nil {
		return nil, err
	}
	result := &IntakeDeliveryResult{Document: doc, Created: created}
	if !created && doc.PayloadHash != n.fingerprint() {
		return result, errxtrace.Classify(ErrIntakeDocumentChanged, errx.Attrs("entity_id", doc.ID))
	}
	if doc.DraftID == nil {
		if err := s.finishDelivery(ctx, set, n, file, result); err != nil {
			return result, err
		}
	}
	if result.State, err = s.documentState(ctx, set, result.Document); err != nil {
		return result, err
	}
	return result, nil
}

// recordDeliveredFile stores the delivered file as a file of the group and
// records it on the document.
func (s *IntakeService) recordDeliveredFile(ctx context.Context, set *registry.Set, n normalizedDelivery, file *IntakeDeliveredFile, result *IntakeDeliveryResult) error {
	stored, err := s.storeDeliveredFile(ctx, set, n, file)
	if err != nil {
		return err
	}
	doc, err := set.IntakeDocumentRegistry.RecordProgress(ctx, result.Document.ID, registry.IntakeDocumentProgress{FileID: &stored.ID})
	if err != nil {
		return err
	}
	result.Document = doc
	if *doc.FileID == stored.ID {
		result.File = stored
		return nil
	}
	// A concurrent delivery of the document recorded its file first; this
	// one is not needed.
	if err := set.FileRegistry.Delete(ctx, stored.ID); err != nil {
		return errxtrace.Wrap("failed to delete the unused delivered file", err)
	}
	return nil
}

// finishDelivery stores the file, makes the intake session and the draft,
// whichever the document does not have yet, and records each on it.
func (s *IntakeService) finishDelivery(ctx context.Context, set *registry.Set, n normalizedDelivery, file *IntakeDeliveredFile, result *IntakeDeliveryResult) error {
	docs := set.IntakeDocumentRegistry
	if file != nil && result.Document.FileID == nil {
		if err := s.recordDeliveredFile(ctx, set, n, file, result); err != nil {
			return err
		}
	}
	doc := result.Document

	session, err := s.deliverySession(ctx, set, n, doc)
	if err != nil {
		return err
	}
	if doc, err = docs.RecordProgress(ctx, doc.ID, registry.IntakeDocumentProgress{SessionID: &session.ID}); err != nil {
		return err
	}
	result.Document = doc

	made, err := s.CreateDrafts(ctx, session.ID)
	if err != nil {
		return err
	}
	draftID, err := deliveredDraftID(made)
	if err != nil || draftID == "" {
		return err
	}
	if result.Document, err = docs.RecordProgress(ctx, doc.ID, registry.IntakeDocumentProgress{DraftID: &draftID}); err != nil {
		return err
	}
	return nil
}

// storeDeliveredFile creates the file entity of a delivered blob.
func (s *IntakeService) storeDeliveredFile(ctx context.Context, set *registry.Set, n normalizedDelivery, file *IntakeDeliveredFile) (*models.FileEntity, error) {
	title := strings.TrimSuffix(file.Name, extOf(file.Name))
	if title == "" {
		title = n.Title
	}
	title = truncateRunes(title, intakeDocumentTitleMaxRunes)
	now := s.now()
	entity := models.FileEntity{
		Title:       title,
		Description: "",
		Type:        models.FileTypeFromMIME(file.MIMEType),
		Category:    models.FileCategoryFromMIME(file.MIMEType),
		Tags:        []string{},
		CreatedAt:   now,
		UpdatedAt:   now,
		File: &models.File{
			Path:         title,
			OriginalPath: file.BlobKey,
			Ext:          mimekit.ExtensionByMime(file.MIMEType),
			MIMEType:     file.MIMEType,
			SizeBytes:    file.Size,
			SHA256:       file.SHA256,
		},
	}
	stored, err := set.FileRegistry.Create(ctx, entity)
	if err != nil {
		return nil, errxtrace.Wrap("failed to store the delivered file", err)
	}
	return stored, nil
}

// deliverySession returns the intake session of the document, making it
// when there is none. The session has one source that is already scanned:
// it carries the external identity, the delivered file and the product the
// delivery describes.
func (s *IntakeService) deliverySession(ctx context.Context, set *registry.Set, n normalizedDelivery, doc *models.IntakeDocument) (*models.IntakeSession, error) {
	reg := set.IntakeRegistry
	clientKey := intakeSessionClientKeyPrefix + doc.ID
	session, err := reg.GetSessionByClientKey(ctx, clientKey)
	if err == nil || !errors.Is(err, registry.ErrNotFound) {
		return session, err
	}
	cand, err := n.candidate()
	if err != nil {
		return nil, err
	}
	var fileIDs []string
	if doc.FileID != nil {
		fileIDs = []string{*doc.FileID}
	}
	session, _, err = reg.CreateSession(ctx,
		models.IntakeSession{ClientKey: clientKey, Origin: models.IntakeOriginExternal},
		[]registry.IntakeSourceInput{{
			FileIDs:            fileIDs,
			OneItem:            true,
			ExternalSystem:     doc.ExternalSystem,
			ExternalInstance:   doc.ExternalInstance,
			ExternalDocumentID: doc.ExternalDocumentID,
			Delivered:          &registry.IntakeScan{Candidates: []models.IntakeCandidate{cand}},
		}},
		0)
	if !errors.Is(err, registry.ErrAlreadyExists) {
		return session, err
	}
	// Another session already carries the identity: one a delivery made
	// before this document record existed, which a restore that kept the
	// sessions but not the documents leaves. It is the document's session.
	src, srcErr := reg.GetSourceByExternalID(ctx, doc.ExternalSystem, doc.ExternalInstance, doc.ExternalDocumentID)
	if srcErr != nil {
		return nil, err
	}
	return reg.Get(ctx, src.SessionID)
}

// deliveredDraftID returns the draft CreateDrafts made for the delivered
// product. A draft with the product's key that is being discarded leaves it
// without one.
func deliveredDraftID(made *IntakeDraftsResult) (string, error) {
	for _, cand := range made.Detail.Candidates {
		switch {
		case cand.DraftID != nil:
			return *cand.DraftID, nil
		case cand.Outcome == models.IntakeCandidateFailed && cand.ErrorCode == IntakeDraftDiscarded:
			return "", nil
		case cand.Outcome == models.IntakeCandidateFailed:
			return "", errxtrace.Classify(registry.ErrInvalidInput, errx.Attrs("error_code", cand.ErrorCode))
		}
	}
	return "", nil
}

// documentState says what became of the document's draft.
func (s *IntakeService) documentState(ctx context.Context, set *registry.Set, doc *models.IntakeDocument) (models.IntakeDocumentState, error) {
	if doc.CommodityID != nil {
		_, err := set.CommodityRegistry.Get(ctx, *doc.CommodityID)
		switch {
		case err == nil:
			return models.IntakeDocumentItem, nil
		case errors.Is(err, registry.ErrNotFound):
			return models.IntakeDocumentRemoved, nil
		default:
			return "", err
		}
	}
	if doc.DraftID == nil {
		return models.IntakeDocumentPending, nil
	}
	draft, err := set.CommodityDraftRegistry.Get(ctx, *doc.DraftID)
	switch {
	case errors.Is(err, registry.ErrNotFound):
		return models.IntakeDocumentRemoved, nil
	case err != nil:
		return "", err
	case draft.Status == models.CommodityDraftOpen:
		return models.IntakeDocumentDraft, nil
	default:
		return models.IntakeDocumentRemoved, nil
	}
}

// DocumentStates returns the state of each document, keyed by its id.
func (s *IntakeService) DocumentStates(ctx context.Context, docs []*models.IntakeDocument) (map[string]models.IntakeDocumentState, error) {
	set, err := s.factorySet.CreateUserRegistrySet(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to create registry set", err)
	}
	states := make(map[string]models.IntakeDocumentState, len(docs))
	for _, doc := range docs {
		if states[doc.ID], err = s.documentState(ctx, set, doc); err != nil {
			return nil, err
		}
	}
	return states, nil
}

// ListDocuments returns the group's delivered documents whose draft is one of
// draftIDs or whose item is one of commodityIDs, oldest first.
func (s *IntakeService) ListDocuments(ctx context.Context, draftIDs, commodityIDs []string) ([]*models.IntakeDocument, error) {
	reg, err := s.factorySet.IntakeDocumentRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to create intake document registry", err)
	}
	out := []*models.IntakeDocument{}
	seen := map[string]bool{}
	byDraft, err := reg.ListByDrafts(ctx, draftIDs...)
	if err != nil {
		return nil, err
	}
	byItem, err := reg.ListByCommodities(ctx, commodityIDs...)
	if err != nil {
		return nil, err
	}
	for _, doc := range append(byDraft, byItem...) {
		if !seen[doc.ID] {
			seen[doc.ID] = true
			out = append(out, doc)
		}
	}
	return out, nil
}

// extOf returns the extension of a file name with its dot, or "".
func extOf(name string) string {
	i := strings.LastIndexByte(name, '.')
	if i <= 0 || strings.ContainsRune(name[i:], '/') {
		return ""
	}
	return name[i:]
}
