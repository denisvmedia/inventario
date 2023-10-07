package boltdb

import (
	bolt "go.etcd.io/bbolt"

	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/boltdb/dbx"
)

type HookFn[T any, P registry.PIDable[T]] func(dbx.TransactionOrBucket, P) error

func NoopHook[T any, P registry.PIDable[T]](P) error {
	return nil
}

type Registry[T any, P registry.PIDable[T]] struct {
	db   *bolt.DB
	base *dbx.BaseRepository[T, P]
}

func NewRegistry[T any, P registry.PIDable[T]](db *bolt.DB, base *dbx.BaseRepository[T, P]) *Registry[T, P] {
	return &Registry[T, P]{
		db:   db,
		base: base,
	}
}

func (r *Registry[T, P]) Create(m T, before, after HookFn[T, P]) (P, error) {
	result := P(&m)

	err := r.db.Update(func(tx *bolt.Tx) error {
		err := before(tx, result)
		if err != nil {
			return err
		}

		result.SetID("") // ignore the id
		err = r.base.Save(tx, result)
		if err != nil {
			return err
		}

		err = after(tx, result)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (r *Registry[T, P]) Get(id string) (result P, err error) {
	err = r.db.View(func(tx *bolt.Tx) error {
		m := P(new(T))
		err := r.base.Get(tx, id, m)
		if err != nil {
			return errkit.Wrap(err, "failed to obtain entity")
		}
		result = m
		return nil
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (r *Registry[T, P]) List() (results []P, err error) {
	err = r.db.View(func(tx *bolt.Tx) error {
		val, err := r.base.GetAll(tx, P(new(T)))
		if err != nil {
			return err
		}
		results = val
		return nil
	})

	if err != nil {
		return nil, err
	}

	return results, nil
}

func (r *Registry[T, P]) Update(m T, before, after HookFn[T, P]) (result P, err error) {
	result = &m
	err = r.db.Update(func(tx *bolt.Tx) error {
		old := P(new(T))

		err := r.base.Get(tx, result.GetID(), old)
		if err != nil {
			return err
		}

		err = before(tx, result)
		if err != nil {
			return err
		}

		err = r.base.Save(tx, result)
		if err != nil {
			return err
		}

		err = after(tx, result)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (r *Registry[T, P]) Count() (int, error) {
	var cnt int

	err := r.db.View(func(tx *bolt.Tx) error {
		var err error
		cnt, err = r.base.Count(tx)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return 0, err
	}

	return cnt, nil
}

func (r *Registry[T, P]) Delete(id string, before, after HookFn[T, P]) error {
	return r.db.Update(func(tx *bolt.Tx) error {
		m := P(new(T))
		err := r.base.Get(tx, id, m)
		if err != nil {
			return err
		}

		err = before(tx, m)
		if err != nil {
			return err
		}

		err = r.base.Delete(tx, id)
		if err != nil {
			return err
		}

		err = after(tx, m)
		if err != nil {
			return err
		}

		return nil
	})
}
