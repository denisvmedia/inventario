package apiserver_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"

	"go.5x5.cz/inventario/apiserver"
	"go.5x5.cz/inventario/jsonapi"
	"go.5x5.cz/inventario/models"
)

func doJSON(c *qt.C, handler http.Handler, userID, method, path, body string) *httptest.ResponseRecorder {
	c.Helper()
	var reader *bytes.Buffer
	if body != "" {
		reader = bytes.NewBufferString(body)
	} else {
		reader = &bytes.Buffer{}
	}
	req := must.Must(http.NewRequest(method, path, reader))
	req.Header.Set("Content-Type", "application/vnd.api+json")
	addTestUserAuthHeader(req, userID)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

func TestCommodityReminders_CRUD(t *testing.T) {
	c := qt.New(t)
	params, testUser, testGroup := newParams()
	regSet := getRegistrySetFromParams(params, testUser)
	ctx := context.Background()
	area := must.Must(regSet.AreaRegistry.List(ctx))[0]
	commodity := must.Must(regSet.CommodityRegistry.Create(ctx, models.Commodity{
		Name:      "Car",
		ShortName: "car",
		AreaID:    new(area.ID),
		Status:    models.CommodityStatusInUse,
		Type:      models.CommodityTypeOther,
		Count:     1,
	}))
	handler := apiserver.APIServer(params, &mockRestoreWorker{})
	base := "/api/v1/g/" + testGroup.Slug

	dueOn := time.Now().UTC().AddDate(0, 0, 20).Format("2006-01-02")
	rr := doJSON(c, handler, testUser.ID, http.MethodPost, base+"/commodities/"+commodity.ID+"/reminders",
		`{"data":{"type":"commodity_reminders","attributes":{"title":"Inspection","due_on":"`+dueOn+`","remind_days_before":[1,14],"repeat_unit":"year"}}}`)
	c.Assert(rr.Code, qt.Equals, http.StatusCreated, qt.Commentf("body=%s", rr.Body.String()))
	var created jsonapi.CommodityReminderResponse
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &created), qt.IsNil)
	attrs := created.Data.Attributes
	c.Assert(attrs.Title, qt.Equals, "Inspection")
	c.Assert([]int(attrs.RemindDaysBefore), qt.DeepEquals, []int{14, 1})
	c.Assert(attrs.RepeatUnit, qt.Equals, models.ReminderRepeatYear)
	c.Assert(attrs.RepeatInterval, qt.Equals, 1)
	c.Assert(attrs.NextOccurrence, qt.Equals, dueOn)
	c.Assert(attrs.DaysUntilNext, qt.IsNotNil)
	c.Assert(*attrs.DaysUntilNext, qt.Equals, 20)
	reminderID := created.Data.ID

	rr = doJSON(c, handler, testUser.ID, http.MethodGet, base+"/commodities/"+commodity.ID+"/reminders", "")
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
	var list jsonapi.CommodityRemindersResponse
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &list), qt.IsNil)
	c.Assert(list.Data, qt.HasLen, 1)
	c.Assert(list.Data[0].ID, qt.Equals, reminderID)

	rr = doJSON(c, handler, testUser.ID, http.MethodPatch, base+"/reminders/"+reminderID,
		`{"data":{"id":"`+reminderID+`","type":"commodity_reminders","attributes":{"title":"Annual inspection","repeat_interval":2}}}`)
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
	var updated jsonapi.CommodityReminderResponse
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &updated), qt.IsNil)
	c.Assert(updated.Data.Attributes.Title, qt.Equals, "Annual inspection")
	c.Assert(updated.Data.Attributes.RepeatInterval, qt.Equals, 2)
	c.Assert([]int(updated.Data.Attributes.RemindDaysBefore), qt.DeepEquals, []int{14, 1})

	rr = doJSON(c, handler, testUser.ID, http.MethodDelete, base+"/reminders/"+reminderID, "")
	c.Assert(rr.Code, qt.Equals, http.StatusNoContent, qt.Commentf("body=%s", rr.Body.String()))
	rr = doJSON(c, handler, testUser.ID, http.MethodDelete, base+"/reminders/"+reminderID, "")
	c.Assert(rr.Code, qt.Equals, http.StatusNotFound)
}

func TestCommodityReminders_RejectsInvalidInput(t *testing.T) {
	params, testUser, testGroup := newParams()
	regSet := getRegistrySetFromParams(params, testUser)
	ctx := context.Background()
	commodity := must.Must(regSet.CommodityRegistry.Create(ctx, models.Commodity{
		Name: "Car", ShortName: "car", Status: models.CommodityStatusInUse, Type: models.CommodityTypeOther, Count: 1,
	}))
	handler := apiserver.APIServer(params, &mockRestoreWorker{})
	path := "/api/v1/g/" + testGroup.Slug + "/commodities/" + commodity.ID + "/reminders"

	tests := []struct {
		name  string
		attrs string
	}{
		{name: "missing title", attrs: `"due_on":"2027-01-01","remind_days_before":[1]`},
		{name: "bad date", attrs: `"title":"x","due_on":"01/01/2027","remind_days_before":[1]`},
		{name: "no lead times", attrs: `"title":"x","due_on":"2027-01-01","remind_days_before":[]`},
		{name: "lead time too large", attrs: `"title":"x","due_on":"2027-01-01","remind_days_before":[366]`},
		{name: "unknown repeat", attrs: `"title":"x","due_on":"2027-01-01","remind_days_before":[1],"repeat_unit":"hourly"}`},
		{name: "interval too large", attrs: `"title":"x","due_on":"2027-01-01","remind_days_before":[1],"repeat_unit":"day","repeat_interval":1000`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			rr := doJSON(c, handler, testUser.ID, http.MethodPost, path,
				`{"data":{"type":"commodity_reminders","attributes":{`+tc.attrs+`}}}`)
			c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity, qt.Commentf("body=%s", rr.Body.String()))
		})
	}
}

func TestUpcoming_MergesSourcesInDateOrder(t *testing.T) {
	c := qt.New(t)
	params, testUser, testGroup := newParams()
	regSet := getRegistrySetFromParams(params, testUser)
	ctx := context.Background()
	today := time.Now().UTC()
	date := func(days int) models.Date { return models.Date(today.AddDate(0, 0, days).Format("2006-01-02")) }

	mk := func(name string, warranty models.PDate) *models.Commodity {
		return must.Must(regSet.CommodityRegistry.Create(ctx, models.Commodity{
			Name: name, ShortName: name, Status: models.CommodityStatusInUse, Type: models.CommodityTypeOther, Count: 1,
			WarrantyExpiresAt: warranty,
		}))
	}
	car := mk("Car", nil)
	mk("TV", new(date(12)))
	drill := mk("Drill", nil)
	boiler := mk("Boiler", nil)
	mk("Far", new(date(200)))

	must.Must(regSet.CommodityReminderRegistry.Create(ctx, models.CommodityReminder{
		CommodityID: car.ID, Title: "Inspection", DueOn: date(5),
		RemindDaysBefore: models.ValuerSlice[int]{7}, RepeatUnit: models.ReminderRepeatNone, RepeatInterval: 1,
	}))
	must.Must(regSet.CommodityLoanRegistry.Create(ctx, models.CommodityLoan{
		CommodityID: drill.ID, BorrowerName: "Anna", LentAt: date(-10), DueBackAt: new(date(-2)),
	}))
	must.Must(regSet.MaintenanceScheduleRegistry.Create(ctx, models.MaintenanceSchedule{
		CommodityID: boiler.ID, Title: "Service", IntervalDays: 365, NextDueAt: date(20), Enabled: true,
	}))

	handler := apiserver.APIServer(params, &mockRestoreWorker{})
	rr := doJSON(c, handler, testUser.ID, http.MethodGet, "/api/v1/g/"+testGroup.Slug+"/upcoming?days=30", "")
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
	var resp jsonapi.UpcomingResponse
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &resp), qt.IsNil)
	c.Assert(resp.Meta.Days, qt.Equals, 30)

	type row struct {
		Kind, Commodity, Title string
		DaysLeft               int
	}
	var got []row
	for _, it := range resp.Data {
		name := ""
		if it.Commodity != nil {
			name = it.Commodity.Name
		}
		got = append(got, row{Kind: it.Kind, Commodity: name, Title: it.Title, DaysLeft: it.DaysLeft})
	}
	c.Assert(got, qt.DeepEquals, []row{
		{Kind: "loan", Commodity: "Drill", Title: "Anna", DaysLeft: -2},
		{Kind: "reminder", Commodity: "Car", Title: "Inspection", DaysLeft: 5},
		{Kind: "warranty", Commodity: "TV", DaysLeft: 12},
		{Kind: "maintenance", Commodity: "Boiler", Title: "Service", DaysLeft: 20},
	})

	rr = doJSON(c, handler, testUser.ID, http.MethodGet, "/api/v1/g/"+testGroup.Slug+"/upcoming?days=0", "")
	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity)
	rr = doJSON(c, handler, testUser.ID, http.MethodGet, "/api/v1/g/"+testGroup.Slug+"/upcoming?days=abc", "")
	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity)
}

func TestSettings_UsesCamelCaseKeys(t *testing.T) {
	c := qt.New(t)
	params, testUser, testGroup := newParams()
	handler := apiserver.APIServer(params, &mockRestoreWorker{})
	path := "/api/v1/g/" + testGroup.Slug + "/settings"

	rr := doJSON(c, handler, testUser.ID, http.MethodPatch, path+"/notifications.item_reminder", `false`)
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))

	rr = doJSON(c, handler, testUser.ID, http.MethodGet, path, "")
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
	var raw map[string]any
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &raw), qt.IsNil)
	c.Assert(raw["notificationsItemReminder"], qt.IsFalse)
	_, hasPascal := raw["NotificationsItemReminder"]
	c.Assert(hasPascal, qt.IsFalse)
	// Unset settings are left out rather than sent as null.
	_, hasTheme := raw["theme"]
	c.Assert(hasTheme, qt.IsFalse)
}
