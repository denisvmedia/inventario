package services

import (
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/jellydator/validation"
	"github.com/shopspring/decimal"

	"go.5x5.cz/inventario/internal/aivision"
	"go.5x5.cz/inventario/internal/filekit"
	"go.5x5.cz/inventario/internal/mimekit"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

// ErrIntakeDocumentChanged is returned when another system delivers a
// document the group already has with different values or a different file.
// The first delivery stands: the draft or item it made is not changed, and no
// second draft is made.
var ErrIntakeDocumentChanged = errx.NewSentinel("the document was delivered before with different values")

// ErrIntakeDocumentConflict is returned when an intake session or a draft
// the group's record of the document did not make carries the document: the
// record of the delivery that made it is gone. Nothing is made.
var ErrIntakeDocumentConflict = errx.NewSentinel("another intake session or draft carries the document")

// intakeDocumentTitleMaxRunes is the draft's limit on a title.
const intakeDocumentTitleMaxRunes = 255

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
	BlobKey string
	// Name is the file name the sender gave. The file's title comes from
	// it, without the extension.
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
// front, the form Paperless-ngx gives a monetary field ("EUR12.30"). The 13
// digits before the point are what an item's price holds; the draft carries
// the amount as a float64, which keeps 15 digits exactly.
var intakeAmountPattern = regexp.MustCompile(`^([A-Za-z]{3})?(\d{1,13}(?:\.\d{1,2})?)$`)

// normalizeIntakeDelivery checks the delivery and puts it in canonical form.
// A malformed value is refused rather than dropped, so the other system sees
// the mistake in its log.
func normalizeIntakeDelivery(in IntakeDelivery, file *IntakeDeliveredFile) (normalizedDelivery, error) {
	if err := checkIntakeText(in); err != nil {
		return normalizedDelivery{}, err
	}
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
	// Bytes: see models.IntakeExternalValueMaxLength.
	if err := validation.Validate(n.SourceSystem, validation.Required, validation.Length(1, models.IntakeExternalValueMaxLength),
		validation.Match(intakeSourceSystemPattern).Error("must be letters, digits, dots, dashes or underscores")); err != nil {
		errs["source_system"] = err
	}
	// Bytes: see models.IntakeExternalValueMaxLength.
	if err := validation.Validate(n.SourceInstance, validation.Length(0, models.IntakeExternalValueMaxLength)); err != nil {
		errs["source_instance"] = err
	}
	// Bytes: see models.IntakeExternalValueMaxLength.
	if err := validation.Validate(n.DocumentID, validation.Required, validation.Length(1, models.IntakeExternalValueMaxLength)); err != nil {
		errs["document_id"] = err
	}
	if err := validation.Validate(n.SourceURL, validation.RuneLength(0, models.IntakeSourceURLMaxLength), models.IntakeSourceURLRule); err != nil {
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

// checkIntakeText refuses a field that is not UTF-8 text or that holds a NUL
// character, which the database cannot store.
func checkIntakeText(in IntakeDelivery) error {
	errs := validation.Errors{}
	for name, value := range map[string]string{
		"title":           in.Title,
		"source_system":   in.SourceSystem,
		"source_instance": in.SourceInstance,
		"document_id":     in.DocumentID,
		"source_url":      in.SourceURL,
		"source_name":     in.SourceName,
		"seller":          in.Seller,
		"purchase_date":   in.PurchaseDate,
		"amount":          in.Amount,
		"currency":        in.Currency,
	} {
		if !utf8.ValidString(value) || strings.ContainsRune(value, 0) {
			errs[name] = validation.NewError("validation_intake_text", "must be UTF-8 text without NUL characters")
		}
	}
	if len(errs) > 0 {
		return errs
	}
	return nil
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
	invalid := validation.NewError("validation_intake_amount",
		"must be a non-negative number with at most 13 digits before the point and two after it, optionally after a currency code")
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
// with other values gets ErrIntakeDocumentChanged and changes nothing, also
// when the first one failed half way: the first values finish it. The
// document record outlives the intake session and the draft, so a replay
// after the member deleted the session, discarded the draft or finished the
// item still makes nothing.
//
// Every step can be repeated: the file and the session are recorded on the
// document once made, and the draft carries a client key that follows from
// the document's identity, so a replay after a failure half way finishes the
// work without doing any of it twice.
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
	delivered := models.IntakeDocument{
		ExternalSystem:     n.SourceSystem,
		ExternalInstance:   n.SourceInstance,
		ExternalDocumentID: n.DocumentID,
		SourceName:         n.SourceName,
		SourceURL:          n.SourceURL,
		PayloadHash:        n.fingerprint(),
	}
	doc, created, err := docs.CreateOrGet(ctx, delivered)
	if err != nil {
		return nil, err
	}
	result := &IntakeDeliveryResult{Document: doc, Created: created}
	if !created && doc.PayloadHash != delivered.PayloadHash {
		return result, errxtrace.Classify(ErrIntakeDocumentChanged, errx.Attrs("entity_id", doc.ID))
	}
	if result.Document.DraftID == nil {
		if err := s.finishDelivery(ctx, set, n, file, result); err != nil {
			return result, dropRefusedRecord(ctx, set, result, err)
		}
	}
	if err := s.dropExtraSession(ctx, set, result.Document); err != nil {
		return result, err
	}
	if err := recordPromotion(ctx, set, result); err != nil {
		return result, err
	}
	if result.State, err = s.documentState(ctx, set, result.Document); err != nil {
		return result, err
	}
	if result.State == models.IntakeDocumentItem && result.Document.CommodityID == nil {
		// The draft became an item after recordPromotion read it.
		if err := recordPromotion(ctx, set, result); err != nil {
			return result, err
		}
	}
	return result, nil
}

// dropRefusedRecord deletes the record that a delivery refused as a conflict
// made, so its values reserve nothing, and the file the delivery stored for
// it, so a refused delivery adds no file to the group. A record another
// delivery of the same values has used since is kept, and so is a file this
// call did not store. It returns err.
func dropRefusedRecord(ctx context.Context, set *registry.Set, result *IntakeDeliveryResult, err error) error {
	if !errors.Is(err, ErrIntakeDocumentConflict) || !result.Created || result.Document == nil {
		return err
	}
	ctx = context.WithoutCancel(ctx)
	docs := set.IntakeDocumentRegistry
	current, getErr := docs.Get(ctx, result.Document.ID)
	switch {
	case errors.Is(getErr, registry.ErrNotFound):
		return err
	case getErr != nil:
		return errors.Join(err, getErr)
	case current.SessionID != nil || current.DraftID != nil || current.CommodityID != nil:
		return err
	case current.FileID != nil && (result.File == nil || *current.FileID != result.File.ID):
		return err
	}
	if delErr := docs.Delete(ctx, result.Document.ID); delErr != nil {
		return errors.Join(err, delErr)
	}
	if current.FileID == nil {
		return err
	}
	stored := result.File
	result.File = nil
	if dropErr := dropDeliveredFile(ctx, set, stored, result); dropErr != nil {
		return errors.Join(err, dropErr)
	}
	return err
}

// dropExtraSession deletes the session a delivery of the document made and
// could not record, since a concurrent delivery recorded its own first. That
// delivery deletes it; when the delete fails, a later delivery of the
// document does. The session the document names is kept.
func (s *IntakeService) dropExtraSession(ctx context.Context, set *registry.Set, doc *models.IntakeDocument) error {
	if doc.SessionID == nil {
		return nil
	}
	session, err := set.IntakeRegistry.GetSessionByClientKey(ctx, models.IntakeDeliverySessionKeyPrefix+doc.ID)
	switch {
	case errors.Is(err, registry.ErrNotFound):
		return nil
	case err != nil:
		return err
	case session.ID == *doc.SessionID:
		return nil
	}
	if err := s.Delete(ctx, session.ID); err != nil && !errors.Is(err, registry.ErrNotFound) {
		return errxtrace.Wrap("failed to delete the session the delivery made again", err)
	}
	return nil
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
		// Unrecorded, the file would be made again by the repeat.
		return errors.Join(err, dropDeliveredFile(ctx, set, stored, result))
	}
	result.Document = doc
	if *doc.FileID == stored.ID {
		result.File = stored
		return nil
	}
	// A concurrent delivery of the document recorded its file first; this
	// one is not needed.
	return dropDeliveredFile(ctx, set, stored, result)
}

// dropDeliveredFile deletes a file entity the delivery made and does not
// use. When that fails the file stays, so result keeps it and the caller
// keeps its blob.
func dropDeliveredFile(ctx context.Context, set *registry.Set, stored *models.FileEntity, result *IntakeDeliveryResult) error {
	if err := set.FileRegistry.Delete(context.WithoutCancel(ctx), stored.ID); err != nil {
		result.File = stored
		return errxtrace.Wrap("failed to delete the unused delivered file", err)
	}
	return nil
}

// finishDelivery makes what the document does not have yet, the file of the
// group, the intake session and the draft, and records each on it.
func (s *IntakeService) finishDelivery(ctx context.Context, set *registry.Set, n normalizedDelivery, file *IntakeDeliveredFile, result *IntakeDeliveryResult) error {
	session, err := s.deliverySession(ctx, set, result)
	if errors.Is(err, errNoDeliverySession) {
		session, err = s.newDeliverySession(ctx, set, n, file, result)
	}
	switch {
	case errors.Is(err, errDeliveryFinished):
		return nil
	case errors.Is(err, errDeliverySessionGone):
		return finishWithoutSession(ctx, set, result)
	case err != nil:
		return err
	}
	doc, err := set.IntakeDocumentRegistry.RecordProgress(ctx, result.Document.ID, registry.IntakeDocumentProgress{SessionID: &session.ID})
	if err != nil {
		return err
	}
	result.Document = doc
	if *doc.SessionID != session.ID {
		// A concurrent delivery recorded its session first, and the member
		// deleted it since: the key of a session is unique, so this one
		// could not be made while that one was there. Making the draft
		// would bring back what the member deleted. Deliver deletes this
		// session (dropExtraSession).
		return finishWithoutSession(ctx, set, result)
	}

	// The session may hold the draft already. Making the drafts again is
	// safe and finishes what a call that stopped half way left, such as
	// attaching the file to the draft.
	made, err := s.CreateDrafts(ctx, session.ID)
	if err != nil {
		return err
	}
	draftID, err := deliveredDraftID(made)
	switch {
	case err != nil:
		return err
	case draftID != "":
		return recordDraft(ctx, set, result, draftID)
	}
	// The draft with the document's key is being discarded.
	_, err = recordDraftByKey(ctx, set, result)
	return err
}

// finishWithoutSession finishes a delivery whose session was deleted. A
// draft the session made stays and is the document's. Without one, a new
// session would bring back what the member deleted, so the delivery is done
// without a draft.
func finishWithoutSession(ctx context.Context, set *registry.Set, result *IntakeDeliveryResult) error {
	if found, err := recordDraftByKey(ctx, set, result); found || err != nil {
		return err
	}
	return recordDraft(ctx, set, result, "")
}

// recordDraft records the draft on the document. A failed write leaves
// result as it was.
func recordDraft(ctx context.Context, set *registry.Set, result *IntakeDeliveryResult, draftID string) error {
	doc, err := set.IntakeDocumentRegistry.RecordProgress(ctx, result.Document.ID, registry.IntakeDocumentProgress{DraftID: &draftID})
	if err != nil {
		return err
	}
	result.Document = doc
	return nil
}

// recordDraftByKey records the draft that carries the document's client
// key, whatever its state, and reports whether there is one.
func recordDraftByKey(ctx context.Context, set *registry.Set, result *IntakeDeliveryResult) (bool, error) {
	draft, err := set.CommodityDraftRegistry.GetByClientKey(ctx, result.Document.DraftClientKey())
	switch {
	case errors.Is(err, registry.ErrNotFound):
		return false, nil
	case err != nil:
		return false, err
	}
	return true, recordDraft(ctx, set, result, draft.ID)
}

// recordPromotion records the item the document's draft became when the
// document does not name it yet: the promotion ran before the document
// recorded the draft.
func recordPromotion(ctx context.Context, set *registry.Set, result *IntakeDeliveryResult) error {
	doc := result.Document
	if doc.CommodityID != nil || doc.DraftID == nil || *doc.DraftID == "" {
		return nil
	}
	draft, err := set.CommodityDraftRegistry.Get(ctx, *doc.DraftID)
	switch {
	case errors.Is(err, registry.ErrNotFound):
		return nil
	case err != nil:
		return err
	case draft.PromotedCommodityID == nil:
		return nil
	}
	stored, err := set.IntakeDocumentRegistry.RecordProgress(ctx, doc.ID, registry.IntakeDocumentProgress{CommodityID: draft.PromotedCommodityID})
	if err != nil {
		return err
	}
	result.Document = stored
	return nil
}

// storeDeliveredFile creates the file entity of a delivered blob.
func (s *IntakeService) storeDeliveredFile(ctx context.Context, set *registry.Set, n normalizedDelivery, file *IntakeDeliveredFile) (*models.FileEntity, error) {
	title := filekit.DisplayName(file.Name)
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

// errDeliverySessionGone reports that the intake session recorded on a
// document no longer exists.
var errDeliverySessionGone = errx.NewSentinel("the intake session of the delivery was deleted")

// errNoDeliverySession reports that no intake session carries the document.
var errNoDeliverySession = errx.NewSentinel("no intake session carries the document")

// errDeliveryFinished reports that the document names its draft: this call
// recorded it, or a concurrent delivery did.
var errDeliveryFinished = errx.NewSentinel("the delivery has its draft")

// deliverySession returns the intake session that carries the delivery: the
// one recorded on the document, or the one a call that stopped before
// recording it made under the document's key.
func (s *IntakeService) deliverySession(ctx context.Context, set *registry.Set, result *IntakeDeliveryResult) (*models.IntakeSession, error) {
	reg := set.IntakeRegistry
	if id := result.Document.SessionID; id != nil {
		session, err := reg.Get(ctx, *id)
		if errors.Is(err, registry.ErrNotFound) {
			return nil, errDeliverySessionGone
		}
		return session, err
	}
	clientKey := models.IntakeDeliverySessionKeyPrefix + result.Document.ID
	session, err := reg.GetSessionByClientKey(ctx, clientKey)
	if !errors.Is(err, registry.ErrNotFound) {
		return session, err
	}
	doc := result.Document
	src, err := reg.GetSourceByExternalID(ctx, doc.ExternalSystem, doc.ExternalInstance, doc.ExternalDocumentID)
	switch {
	case err == nil:
		return sessionOfSource(ctx, reg, src, clientKey, doc)
	case !errors.Is(err, registry.ErrNotFound):
		return nil, errxtrace.Wrap("failed to look up the source of the document", err)
	}
	return nil, errNoDeliverySession
}

// newDeliverySession goes on with a document that no session carries. It
// stores the file and makes the session, whose one source is already
// scanned: it carries the external identity, the delivered file and the
// product the delivery describes.
//
// A draft with the document's key can be there without a session: a restore
// brings back the record and the draft, not the session, and the draft is
// the record's. On a record this call made, the draft is the one of an
// earlier delivery whose record the group no longer has. What that delivery
// held is not known, so the delivery is refused rather than take the draft
// over.
func (s *IntakeService) newDeliverySession(ctx context.Context, set *registry.Set, n normalizedDelivery, file *IntakeDeliveredFile, result *IntakeDeliveryResult) (*models.IntakeSession, error) {
	draft, err := set.CommodityDraftRegistry.GetByClientKey(ctx, result.Document.DraftClientKey())
	if err != nil && !errors.Is(err, registry.ErrNotFound) {
		return nil, err
	}
	// A concurrent delivery of the document may have recorded its session
	// or its draft since this call read the document. It records the
	// session before it makes the draft, so a draft found above comes with
	// its session in this read.
	current, err := set.IntakeDocumentRegistry.Get(ctx, result.Document.ID)
	if err != nil {
		return nil, err
	}
	result.Document = current
	switch {
	case result.Document.DraftID != nil:
		return nil, errDeliveryFinished
	case result.Document.SessionID != nil:
		return s.deliverySession(ctx, set, result)
	case draft != nil && result.Created:
		return nil, errxtrace.Classify(ErrIntakeDocumentConflict, errx.Attrs("entity_id", result.Document.ID))
	case draft != nil:
		if err := recordDraft(ctx, set, result, draft.ID); err != nil {
			return nil, err
		}
		return nil, errDeliveryFinished
	}
	if file != nil && result.Document.FileID == nil {
		if err := s.recordDeliveredFile(ctx, set, n, file, result); err != nil {
			return nil, err
		}
		if result.Document.SessionID != nil {
			return s.deliverySession(ctx, set, result)
		}
	}
	doc := result.Document
	cand, err := n.candidate()
	if err != nil {
		return nil, err
	}
	var fileIDs []string
	if doc.FileID != nil {
		fileIDs = []string{*doc.FileID}
	}
	reg := set.IntakeRegistry
	clientKey := models.IntakeDeliverySessionKeyPrefix + doc.ID
	session, _, err := reg.CreateSession(ctx,
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
	if errors.Is(err, registry.ErrAlreadyExists) {
		// A source with the identity appeared since the look-up.
		src, err := reg.GetSourceByExternalID(ctx, doc.ExternalSystem, doc.ExternalInstance, doc.ExternalDocumentID)
		if err != nil {
			return nil, errxtrace.Wrap("failed to look up the source of the document", err)
		}
		return sessionOfSource(ctx, reg, src, clientKey, doc)
	}
	return session, err
}

// sessionOfSource returns the session of a source that carries the
// document's identity when a concurrent delivery of the document made it
// under clientKey. A session made for another record is the one of an
// earlier delivery whose record the group no longer has. What that delivery
// held is not known, so this one is refused rather than take it over.
func sessionOfSource(ctx context.Context, reg registry.IntakeRegistry, src *models.IntakeSource, clientKey string, doc *models.IntakeDocument) (*models.IntakeSession, error) {
	session, err := reg.Get(ctx, src.SessionID)
	switch {
	case err == nil && session.ClientKey == clientKey:
		return session, nil
	case err != nil && !errors.Is(err, registry.ErrNotFound):
		return nil, err
	}
	return nil, errxtrace.Classify(ErrIntakeDocumentConflict, errx.Attrs("entity_id", doc.ID))
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

// documentState says what became of the document's draft. A promoted draft
// gives the item it became also when the document does not name it.
func (s *IntakeService) documentState(ctx context.Context, set *registry.Set, doc *models.IntakeDocument) (models.IntakeDocumentState, error) {
	commodityID := doc.CommodityID
	if commodityID == nil {
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
		case draft.PromotedCommodityID == nil:
			return models.IntakeDocumentRemoved, nil
		}
		commodityID = draft.PromotedCommodityID
	}
	_, err := set.CommodityRegistry.Get(ctx, *commodityID)
	switch {
	case err == nil:
		return models.IntakeDocumentItem, nil
	case errors.Is(err, registry.ErrNotFound):
		return models.IntakeDocumentRemoved, nil
	default:
		return "", err
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
	slices.SortFunc(out, func(x, y *models.IntakeDocument) int {
		return cmp.Or(x.CreatedAt.Compare(y.CreatedAt), cmp.Compare(x.ID, y.ID))
	})
	return out, nil
}
