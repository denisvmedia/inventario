package services

import (
	"fmt"
	"testing"

	"github.com/alicebob/miniredis/v2"
	qt "github.com/frankban/quicktest"

	emailqueueinmemory "github.com/denisvmedia/inventario/email/queue/inmemory"
	emailqueueredis "github.com/denisvmedia/inventario/email/queue/redis"
)

func TestNewEmailQueue_WithoutRedisURL_UsesInMemory(t *testing.T) {
	c := qt.New(t)

	q := newEmailQueue("")
	_, ok := q.(*emailqueueinmemory.Queue)
	c.Assert(ok, qt.IsTrue)
}

func TestNewEmailQueue_InvalidRedisURL_FallsBackToInMemory(t *testing.T) {
	c := qt.New(t)

	q := newEmailQueue("://invalid")
	_, ok := q.(*emailqueueinmemory.Queue)
	c.Assert(ok, qt.IsTrue)
}

func TestNewEmailQueue_ValidRedisURL_UsesRedisQueue(t *testing.T) {
	c := qt.New(t)

	mr, err := miniredis.Run()
	c.Assert(err, qt.IsNil)
	defer mr.Close()

	q := newEmailQueue(fmt.Sprintf("redis://%s/0", mr.Addr()))
	_, ok := q.(*emailqueueredis.Queue)
	c.Assert(ok, qt.IsTrue)
}
