package jsonapi_test

import (
	"encoding/json"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/jsonapi"
)

// TestCommodityLoanUpdateRequestData_UnmarshalJSON_DueBackAt locks the
// presence-aware decode introduced for issue #1513. The three cases the
// FE relies on are:
//   - absent key  → DueBackAt nil + ClearDueBackAt false  (leave unchanged)
//   - JSON null   → DueBackAt nil + ClearDueBackAt true   (clear column)
//   - "YYYY-MM-DD" → DueBackAt set + ClearDueBackAt false (set column)
func TestCommodityLoanUpdateRequestData_UnmarshalJSON_DueBackAt(t *testing.T) {
	t.Run("absent key leaves both flags zero", func(t *testing.T) {
		c := qt.New(t)
		var d jsonapi.CommodityLoanUpdateRequestData
		err := json.Unmarshal([]byte(`{"borrower_name":"Alice"}`), &d)
		c.Assert(err, qt.IsNil)
		c.Assert(d.DueBackAt, qt.IsNil)
		c.Assert(d.ClearDueBackAt, qt.IsFalse)
	})

	t.Run("explicit null sets ClearDueBackAt", func(t *testing.T) {
		c := qt.New(t)
		var d jsonapi.CommodityLoanUpdateRequestData
		err := json.Unmarshal([]byte(`{"due_back_at":null}`), &d)
		c.Assert(err, qt.IsNil)
		c.Assert(d.DueBackAt, qt.IsNil)
		c.Assert(d.ClearDueBackAt, qt.IsTrue)
	})

	t.Run("date string populates DueBackAt without clear flag", func(t *testing.T) {
		c := qt.New(t)
		var d jsonapi.CommodityLoanUpdateRequestData
		err := json.Unmarshal([]byte(`{"due_back_at":"2026-12-31"}`), &d)
		c.Assert(err, qt.IsNil)
		c.Assert(d.DueBackAt, qt.IsNotNil)
		c.Assert(string(*d.DueBackAt), qt.Equals, "2026-12-31")
		c.Assert(d.ClearDueBackAt, qt.IsFalse)
	})

	t.Run("malformed date still rejected", func(t *testing.T) {
		c := qt.New(t)
		var d jsonapi.CommodityLoanUpdateRequestData
		err := json.Unmarshal([]byte(`{"due_back_at":"not-a-date"}`), &d)
		c.Assert(err, qt.IsNotNil)
	})

	t.Run("null with whitespace still detected", func(t *testing.T) {
		// json.RawMessage preserves whitespace inside the value;
		// TrimSpace + bytes.Equal in UnmarshalJSON guards the
		// pretty-printed-body case.
		c := qt.New(t)
		var d jsonapi.CommodityLoanUpdateRequestData
		err := json.Unmarshal([]byte(`{"due_back_at":   null   }`), &d)
		c.Assert(err, qt.IsNil)
		c.Assert(d.ClearDueBackAt, qt.IsTrue)
	})

	t.Run("other patch fields decode normally alongside null due_back_at", func(t *testing.T) {
		c := qt.New(t)
		var d jsonapi.CommodityLoanUpdateRequestData
		err := json.Unmarshal(
			[]byte(`{"borrower_contact":"alice@new.example.com","due_back_at":null}`),
			&d,
		)
		c.Assert(err, qt.IsNil)
		c.Assert(d.BorrowerContact, qt.IsNotNil)
		c.Assert(*d.BorrowerContact, qt.Equals, "alice@new.example.com")
		c.Assert(d.ClearDueBackAt, qt.IsTrue)
	})
}
