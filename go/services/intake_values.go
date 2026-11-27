package services

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
	"math"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jellydator/validation"
	"github.com/shopspring/decimal"

	"go.5x5.cz/inventario/internal/aivision"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

// IntakeAcceptConfidence is the confidence from which a value the scan read
// is accepted by default. It matches the single-item scan review, which
// pre-checks the same values.
const IntakeAcceptConfidence = 0.3

// intakeShortNameMaxRunes is the item form's limit on the short name.
const intakeShortNameMaxRunes = 40

// IntakeReviewFields are the scan fields a member can accept into a draft.
// document_type and quantity are not among them: the first describes the
// file, and the quantity is reviewed as a number of its own.
var IntakeReviewFields = []string{
	aivision.FieldNameName,
	aivision.FieldNameShortName,
	aivision.FieldNameBrand,
	aivision.FieldNameModel,
	aivision.FieldNameSeller,
	aivision.FieldNameType,
	aivision.FieldNameOriginalPrice,
	aivision.FieldNameOriginalPriceCurrency,
	aivision.FieldNameSerialNumber,
	aivision.FieldNameURLs,
	aivision.FieldNamePurchaseDate,
	aivision.FieldNameWarrantyExpiresAt,
	aivision.FieldNameComments,
	aivision.FieldNameTags,
}

func isIntakeReviewField(name string) bool {
	return slices.Contains(IntakeReviewFields, name)
}

// intakeCandidatesFromScan turns a scan result into the candidates of a
// source. A multi-product result gives one candidate per product in the
// order the provider listed them, most prominent first. Products the scan
// read nothing for are left out. The custom field guesses are those the
// scan kept for the group's fields in defs, a select value already an option
// ID; a value an item could not store is left out, so the review never
// offers it.
func intakeCandidatesFromScan(result *aivision.ScanResult, defs map[string]*models.CustomFieldDefinition) []models.IntakeCandidate {
	if result == nil {
		return nil
	}
	items := result.Items
	if len(items) == 0 && (len(result.Fields) > 0 || len(result.CustomFields) > 0) {
		items = []aivision.ScanItem{{Fields: result.Fields, CustomFields: result.CustomFields}}
	}
	out := make([]models.IntakeCandidate, 0, len(items))
	for _, item := range items {
		fields := intakeGuesses(item.Fields)
		custom := storableCustomGuesses(defs, intakeGuesses(item.CustomFields))
		if !hasReviewValue(fields) && len(custom) == 0 {
			continue
		}
		accepted := []string{}
		for _, name := range IntakeReviewFields {
			if guess, ok := fields[name]; ok && guess.Confidence >= IntakeAcceptConfidence {
				accepted = append(accepted, name)
			}
		}
		acceptedCustom := []string{}
		for _, id := range slices.Sorted(maps.Keys(custom)) {
			if custom[id].Confidence >= IntakeAcceptConfidence {
				acceptedCustom = append(acceptedCustom, id)
			}
		}
		out = append(out, models.IntakeCandidate{
			Fields:               fields,
			CustomFields:         custom,
			Selected:             true,
			AcceptedFields:       accepted,
			AcceptedCustomFields: acceptedCustom,
			Quantity:             quantityFromGuess(fields[aivision.FieldNameQuantity]),
		})
	}
	return out
}

// intakeGuesses keeps the guesses that hold a value. A NUL character is
// removed from scanned text, since PostgreSQL cannot store it in JSONB.
func intakeGuesses(guesses map[string]aivision.FieldGuess) models.IntakeFields {
	out := models.IntakeFields{}
	for name, guess := range guesses {
		if guess.Value != nil {
			out[name] = models.IntakeFieldGuess{Value: withoutNUL(guess.Value), Confidence: guess.Confidence}
		}
	}
	return out
}

// withoutNUL removes NUL characters from a scanned string or list of
// strings.
func withoutNUL(v any) any {
	switch v := v.(type) {
	case string:
		return strings.ReplaceAll(v, "\x00", "")
	case []any:
		out := make([]any, len(v))
		for i, e := range v {
			out[i] = withoutNUL(e)
		}
		return out
	case []string:
		out := make([]string, len(v))
		for i, e := range v {
			out[i] = strings.ReplaceAll(e, "\x00", "")
		}
		return out
	}
	return v
}

// storableCustomGuesses keeps the custom field guesses an item could store:
// the field is in defs and its value parses as that field's value.
func storableCustomGuesses(defs map[string]*models.CustomFieldDefinition, guesses models.IntakeFields) models.IntakeFields {
	out := models.IntakeFields{}
	for id, guess := range guesses {
		if _, ok := storedCustomFieldValue(defs[id], guess.Value); ok {
			out[id] = guess
		}
	}
	return out
}

// hasReviewValue reports whether the scan read a built-in field a draft can
// use.
func hasReviewValue(fields models.IntakeFields) bool {
	for _, name := range IntakeReviewFields {
		if _, ok := fields[name]; ok {
			return true
		}
	}
	return false
}

// quantityFromGuess reads the scanned number of units, 1 when there is none
// or it is not a sensible count.
func quantityFromGuess(guess models.IntakeFieldGuess) int {
	n, ok := guess.Value.(float64)
	if !ok || math.IsNaN(n) || math.IsInf(n, 0) {
		return 1
	}
	q := int(math.Round(n))
	if q < 1 || q > models.IntakeCandidateQuantityMax {
		return 1
	}
	return q
}

// intakeDraftInput is what turning a candidate into draft values needs from
// the group.
type intakeDraftInput struct {
	sellers registry.SellerRegistry
	types   registry.ItemTypeRegistry
	// customFields are the group's custom fields by ID.
	customFields map[string]*models.CustomFieldDefinition
}

// intakeDraftValues builds the values of the draft made for a candidate from
// the fields the member accepted, the way the single-item scan fills the
// item form. A value the form could not use is left out, so the field stays
// unknown rather than wrong: a date that is not YYYY-MM-DD, an unknown
// currency, a type the group does not have (#2802), a price that is not a
// number, a custom field value an item could not store. The price is the
// product's own line amount; nothing else is ever copied into it.
func intakeDraftValues(ctx context.Context, in intakeDraftInput, cand *models.IntakeCandidate) (models.CommodityDraftValues, error) {
	values := models.CommodityDraftValues{}
	setTextValues(values, cand)
	if err := setTypeValue(ctx, values, in.types, cand); err != nil {
		return nil, err
	}
	setPriceValues(values, cand)
	for _, name := range []string{aivision.FieldNamePurchaseDate, aivision.FieldNameWarrantyExpiresAt} {
		if s, ok := acceptedText(cand, name); ok {
			if _, err := time.Parse(time.DateOnly, s); err == nil {
				values[name] = s
			}
		}
	}
	if urls := acceptedList(cand, aivision.FieldNameURLs); len(urls) > 0 {
		values["urls"] = urls
	}
	if tags := dedupeFold(acceptedList(cand, aivision.FieldNameTags)); len(tags) > 0 {
		values["tags"] = tags
	}
	if err := setSellerValue(ctx, values, in.sellers, cand); err != nil {
		return nil, err
	}
	if custom := acceptedCustomFieldValues(in.customFields, cand); len(custom) > 0 {
		values["custom_fields"] = custom
	}
	if cand.Quantity > 1 {
		values["count"] = float64(cand.Quantity)
		// A count above one is a bundle of identical units, which cannot
		// hold a warranty; the date would block finishing the item.
		delete(values, aivision.FieldNameWarrantyExpiresAt)
	}
	return values, nil
}

// acceptedText returns an accepted text field, trimmed, when it is not
// empty.
func acceptedText(cand *models.IntakeCandidate, name string) (string, bool) {
	if !cand.Accepts(name) {
		return "", false
	}
	s, ok := cand.Fields[name].Value.(string)
	s = strings.TrimSpace(s)
	return s, ok && s != ""
}

// setTextValues copies the accepted free-text fields, cut to the item
// form's limits.
func setTextValues(values models.CommodityDraftValues, cand *models.IntakeCandidate) {
	for _, name := range []string{aivision.FieldNameName, aivision.FieldNameSerialNumber, aivision.FieldNameComments} {
		if s, ok := acceptedText(cand, name); ok {
			values[name] = s
		}
	}
	limits := []struct {
		field, key string
		max        int
	}{
		{aivision.FieldNameShortName, "short_name", intakeShortNameMaxRunes},
		{aivision.FieldNameBrand, "brand", models.MaxCommodityBrandRunes},
		{aivision.FieldNameModel, "model", models.MaxCommodityModelRunes},
	}
	for _, l := range limits {
		if s, ok := acceptedText(cand, l.field); ok {
			values[l.key] = truncateRunes(s, l.max)
		}
	}
}

// setPriceValues copies the accepted line amount and its currency. An
// amount that is not a non-negative number, or a currency the app does not
// know, stays unknown.
func setPriceValues(values models.CommodityDraftValues, cand *models.IntakeCandidate) {
	if s, ok := acceptedText(cand, aivision.FieldNameOriginalPriceCurrency); ok {
		if code := models.Currency(strings.ToUpper(s)); code.IsValid() {
			values["original_price_currency"] = string(code)
		}
	}
	if !cand.Accepts(aivision.FieldNameOriginalPrice) {
		return
	}
	if n, ok := cand.Fields[aivision.FieldNameOriginalPrice].Value.(float64); ok && !math.IsNaN(n) && !math.IsInf(n, 0) && n >= 0 {
		values["original_price"] = decimal.NewFromFloat(n).StringFixed(2)
	}
}

// setTypeValue copies the accepted type when the group has it.
func setTypeValue(ctx context.Context, values models.CommodityDraftValues, types registry.ItemTypeRegistry, cand *models.IntakeCandidate) error {
	s, ok := acceptedText(cand, aivision.FieldNameType)
	if !ok {
		return nil
	}
	err := CheckCommodityType(ctx, types, models.CommodityType(s))
	if _, unknown := errors.AsType[validation.Error](err); unknown {
		return nil
	}
	if err != nil {
		return err
	}
	values["type"] = s
	return nil
}

// setSellerValue names the group's seller with the scanned name. When the
// group has no such seller, the name waits in the draft for the member to
// create the seller when finishing the item.
func setSellerValue(ctx context.Context, values models.CommodityDraftValues, sellers registry.SellerRegistry, cand *models.IntakeCandidate) error {
	s, ok := acceptedText(cand, aivision.FieldNameSeller)
	if !ok {
		return nil
	}
	name := truncateRunes(strings.Join(strings.Fields(s), " "), models.MaxSellerNameRunes)
	seller, err := sellers.GetByName(ctx, name)
	switch {
	case err == nil:
		values["seller_id"] = seller.ID
	case errors.Is(err, registry.ErrNotFound):
		values[IntakeDraftSellerNameKey] = name
	default:
		return err
	}
	return nil
}

// acceptedCustomFieldValues returns the accepted custom field values in the
// form an item stores them, by field ID. A value is left out when its field
// is gone, when the item could not store it (an option the field no longer
// has, a date that is not YYYY-MM-DD, a text over the limit), or when it is
// empty.
func acceptedCustomFieldValues(defs map[string]*models.CustomFieldDefinition, cand *models.IntakeCandidate) map[string]any {
	out := map[string]any{}
	for _, id := range cand.AcceptedCustomFields {
		guess, ok := cand.CustomFields[id]
		if !ok {
			continue
		}
		if v, ok := storedCustomFieldValue(defs[id], guess.Value); ok {
			out[id] = v
		}
	}
	return out
}

// storedCustomFieldValue returns value in the form an item stores it for
// field def, and false when there is no field or the item could not store
// the value, or it is empty.
func storedCustomFieldValue(def *models.CustomFieldDefinition, value any) (any, bool) {
	if def == nil {
		return nil, false
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, false
	}
	data, err := def.ParseValue(raw)
	if err != nil {
		return nil, false
	}
	var v any
	if stored := data.JSON(def.Type); stored == nil || json.Unmarshal(stored, &v) != nil || v == "" {
		return nil, false
	}
	return v, true
}

// IntakeDraftSellerNameKey is the draft value that carries a scanned seller
// name the group does not have yet. The item form offers it as the seller to
// create.
const IntakeDraftSellerNameKey = "seller_name"

// acceptedList reads an accepted list field as the []any of strings a
// draft stores.
func acceptedList(cand *models.IntakeCandidate, name string) []any {
	if !cand.Accepts(name) {
		return nil
	}
	var raw []any
	switch v := cand.Fields[name].Value.(type) {
	case []any:
		raw = v
	case []string:
		for _, s := range v {
			raw = append(raw, s)
		}
	default:
		return nil
	}
	out := []any{}
	for _, v := range raw {
		s, ok := v.(string)
		s = strings.TrimSpace(s)
		if ok && s != "" {
			out = append(out, s)
		}
	}
	return out
}

// dedupeFold drops repeated strings regardless of case, keeping the first.
func dedupeFold(list []any) []any {
	seen := map[string]bool{}
	out := make([]any, 0, len(list))
	for _, v := range list {
		key := strings.ToLower(v.(string))
		if !seen[key] {
			seen[key] = true
			out = append(out, v)
		}
	}
	return out
}

func truncateRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return strings.TrimSpace(string([]rune(s)[:n]))
}
