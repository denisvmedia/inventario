package models

import (
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"golang.org/x/crypto/bcrypt"
)

// EqualizePasswordTiming exists to make the unknown-email login path cost
// what the wrong-password path costs (#2246). Two things have to hold for
// that: it must actually run a bcrypt comparison at the cost SetPassword
// uses, and it must not pay to generate a hash on every call — generating
// one costs about as much as the comparison it is equalizing, which would
// make the "no such user" path measurably SLOWER than the real one.
func TestEqualizePasswordTiming(t *testing.T) {
	c := qt.New(t)
	SetBcryptCostForTesting(t, bcrypt.MinCost)
	dummyHashes.Delete(int32(bcrypt.MinCost))

	EqualizePasswordTiming("whatever")

	cached, ok := dummyHashes.Load(int32(bcrypt.MinCost))
	c.Assert(ok, qt.IsTrue, qt.Commentf("the dummy hash must be cached per cost"))
	cost, err := bcrypt.Cost(cached.([]byte))
	c.Assert(err, qt.IsNil)
	c.Assert(cost, qt.Equals, bcrypt.MinCost,
		qt.Commentf("a dummy at a different cost would not equalize anything"))

	// A second call reuses the cached hash rather than generating another.
	EqualizePasswordTiming("whatever")
	again, _ := dummyHashes.Load(int32(bcrypt.MinCost))
	c.Assert(string(again.([]byte)), qt.Equals, string(cached.([]byte)))
}

// A cost change — which only tests make — must not keep equalizing against
// the old cost's hash.
func TestEqualizePasswordTimingFollowsCost(t *testing.T) {
	c := qt.New(t)
	SetBcryptCostForTesting(t, bcrypt.MinCost+1)
	dummyHashes.Delete(int32(bcrypt.MinCost + 1))

	EqualizePasswordTiming("whatever")

	cached, ok := dummyHashes.Load(int32(bcrypt.MinCost + 1))
	c.Assert(ok, qt.IsTrue)
	cost, err := bcrypt.Cost(cached.([]byte))
	c.Assert(err, qt.IsNil)
	c.Assert(cost, qt.Equals, bcrypt.MinCost+1)
}

// The whole point is the elapsed time, so assert the floor holds: the call
// must not return instantly the way the unequalized branch did.
func TestEqualizePasswordTimingSpendsWork(t *testing.T) {
	c := qt.New(t)
	SetBcryptCostForTesting(t, bcrypt.MinCost+4)
	EqualizePasswordTiming("warm the cache")

	start := time.Now()
	EqualizePasswordTiming("whatever")
	c.Assert(time.Since(start) > time.Millisecond, qt.IsTrue,
		qt.Commentf("a no-op equalizer leaves the enumeration oracle open"))
}
