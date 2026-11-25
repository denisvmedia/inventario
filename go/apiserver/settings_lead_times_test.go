package apiserver_test

import (
	"encoding/json"
	"net/http"
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/apiserver"
	"go.5x5.cz/inventario/debug"
)

// Per-user lead times for the built-in reminders (#2805).

func TestSettings_LeadTimes(t *testing.T) {
	c := qt.New(t)
	params, testUser, testGroup := newParams()
	handler := apiserver.APIServer(params, &mockRestoreWorker{})
	path := "/api/v1/g/" + testGroup.Slug + "/settings"

	getSettings := func() map[string]any {
		rr := doJSON(c, handler, testUser.ID, http.MethodGet, path, "")
		c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
		var raw map[string]any
		c.Assert(json.Unmarshal(rr.Body.Bytes(), &raw), qt.IsNil)
		return raw
	}

	rr := doJSON(c, handler, testUser.ID, http.MethodPatch, path+"/notifications.warranty_lead_times", `[90, 30]`)
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
	rr = doJSON(c, handler, testUser.ID, http.MethodPatch, path+"/notifications.maintenance_lead_times", `{"value": [21]}`)
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
	rr = doJSON(c, handler, testUser.ID, http.MethodPatch, path+"/notifications.loan_lead_time", `14`)
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))

	raw := getSettings()
	c.Assert(raw["notificationsWarrantyLeadTimes"], qt.DeepEquals, []any{float64(90), float64(30)})
	c.Assert(raw["notificationsMaintenanceLeadTimes"], qt.DeepEquals, []any{float64(21)})
	c.Assert(raw["notificationsLoanLeadTime"], qt.Equals, float64(14))

	for _, tc := range []struct {
		field, body string
	}{
		{"notifications.warranty_lead_times", `[0]`},
		{"notifications.warranty_lead_times", `[400]`},
		{"notifications.warranty_lead_times", `[7, 7]`},
		{"notifications.warranty_lead_times", `[]`},
		{"notifications.warranty_lead_times", `[7, "soon"]`},
		{"notifications.maintenance_lead_times", `[1, 2, 3, 4, 5, 6]`},
		{"notifications.loan_lead_time", `0`},
		{"notifications.loan_lead_time", `61`},
		{"notifications.loan_lead_time", `7.5`},
		{"notifications.warranty_expiry", `"yes"`},
	} {
		rr := doJSON(c, handler, testUser.ID, http.MethodPatch, path+"/"+tc.field, tc.body)
		c.Check(rr.Code, qt.Equals, http.StatusBadRequest, qt.Commentf("%s <- %s: %s", tc.field, tc.body, rr.Body.String()))
	}
	// A refused value leaves the stored one alone.
	c.Assert(getSettings()["notificationsWarrantyLeadTimes"], qt.DeepEquals, []any{float64(90), float64(30)})

	// A bare null goes back to the default, which the response leaves out.
	rr = doJSON(c, handler, testUser.ID, http.MethodPatch, path+"/notifications.warranty_lead_times", `null`)
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
	_, has := getSettings()["notificationsWarrantyLeadTimes"]
	c.Assert(has, qt.IsFalse)
}

func TestSystem_ReportsTheLoanLeadTimeDefault(t *testing.T) {
	c := qt.New(t)
	params, testUser, _ := newParams()
	params.LoanReminderDueSoonDays = 5
	params.DebugInfo = debug.NewInfo("memory://", "file:///tmp")
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	rr := doJSON(c, handler, testUser.ID, http.MethodGet, "/api/v1/system", "")
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
	var raw map[string]any
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &raw), qt.IsNil)
	c.Assert(raw["loan_reminder_due_soon_days"], qt.Equals, float64(5))
}
