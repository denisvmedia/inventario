package registrytest

import (
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/registry"
)

// RunCountEachContract checks that CommodityRegistry.CountEach reads every
// count of a batch from one snapshot: an item moved to Trash and back while
// the batch runs is counted once, live or in Trash, never twice or not at
// all. newEnv is called once per subtest and must return an empty group.
func RunCountEachContract(t *testing.T, newEnv func(t *testing.T) TrashEnv) {
	t.Run("a batch counts a moving item exactly once", func(t *testing.T) {
		c := qt.New(t)
		env := newEnv(t)
		ctx := env.Ctx
		item := trashContractItem(c, env, "Ladder")
		opts := []registry.CommodityListOptions{
			{IncludeInactive: true},
			{IncludeInactive: true, Trashed: true},
		}

		done := make(chan struct{})
		moved := make(chan error, 1)
		go func() {
			defer close(moved)
			for {
				select {
				case <-done:
					return
				default:
				}
				if _, err := env.Commodities.Trash(ctx, item.ID, "registrytest", time.Now()); err != nil {
					moved <- err
					return
				}
				if _, err := env.Commodities.Restore(ctx, item.ID); err != nil {
					moved <- err
					return
				}
			}
		}()
		defer func() {
			select {
			case <-done:
			default:
				close(done)
			}
			<-moved
		}()

		for range 300 {
			counts, err := env.Commodities.CountEach(ctx, opts)
			c.Assert(err, qt.IsNil)
			c.Assert(counts[0]+counts[1], qt.Equals, 1,
				qt.Commentf("live %d + Trash %d: the item was counted %d times", counts[0], counts[1], counts[0]+counts[1]))
		}
		close(done)
		c.Assert(<-moved, qt.IsNil)
	})
}
