// Package utcclock pins the process to UTC.
package utcclock

import "time"

// Pin sets time.Local to UTC, so every time.Now() in the process carries a UTC
// wall clock.
//
// Almost every timestamp column in the schema is `timestamp without time
// zone`: PostgreSQL keeps the wall clock and drops the offset. A value written
// from a host at +02:00 is stored two hours ahead of the instant it meant and
// reads back as if it were UTC, which also puts it two hours out from the
// now() an expiry comparison in SQL uses. West of UTC it goes the other way
// and tokens expire early.
//
// Normalizing at each write would work until the next call site. There are
// already both kinds in registry/postgres — some pass time.Now().UTC() and
// some pass time.Now() — which is how the two came to disagree. Pinning the
// process covers every path, including the ones that hand a time.Time straight
// to the driver.
//
// The shipped container has no TZ set and its base image is UTC, so this
// changes nothing for it and closes the gap for everything else. #2594
func Pin() {
	time.Local = time.UTC
}
