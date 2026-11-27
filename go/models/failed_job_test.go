package models_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/models"
)

// A personal data export is a failed-job type of the system-admin plane but
// not of a group: it belongs to a user.
func TestFailedJobGroupTypes_LeaveOutPersonalDataExports(t *testing.T) {
	c := qt.New(t)

	c.Assert(models.FailedJobTypes(), qt.Contains, models.FailedJobTypePersonalDataExport)
	c.Assert(models.IsFailedJobType(models.FailedJobTypePersonalDataExport), qt.IsTrue)
	c.Assert(models.FailedJobRetryable(models.FailedJobTypePersonalDataExport), qt.IsTrue)

	c.Assert(models.FailedJobGroupTypes(), qt.Not(qt.Contains), models.FailedJobTypePersonalDataExport)
	c.Assert(models.IsFailedJobGroupType(models.FailedJobTypePersonalDataExport), qt.IsFalse)
	c.Assert(models.FailedJobGroupTypes(), qt.HasLen, len(models.FailedJobTypes())-1)
	for _, jobType := range models.FailedJobGroupTypes() {
		c.Assert(models.IsFailedJobGroupType(jobType), qt.IsTrue, qt.Commentf("%s", jobType))
	}
}

// The failed-job type of personal data exports is not a worker that can be
// paused: the export worker builds them.
func TestFailedJobTypePersonalDataExport_IsNoPausableWorker(t *testing.T) {
	c := qt.New(t)

	_, ok := models.ParseWorkerType(string(models.FailedJobTypePersonalDataExport))
	c.Assert(ok, qt.IsFalse)
	c.Assert(models.IsFailedJobGroupType("weekly-digest"), qt.IsFalse)
}
