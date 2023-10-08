package boltdb

import (
	"fmt"

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
	db                 *bolt.DB
	base               *dbx.BaseRepository[T, P]
	entityName         string
	childrenBucketName string
}

func NewRegistry[T any, P registry.PIDable[T]](
	db *bolt.DB,
	base *dbx.BaseRepository[T, P],
	entityName string,
	childrenBucketName string,
) *Registry[T, P] {
	return &Registry[T, P]{
		db:                 db,
		base:               base,
		entityName:         entityName,
		childrenBucketName: childrenBucketName,
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

func (r *Registry[T, P]) GetBy(idx, value string) (result P, err error) {
	err = r.db.View(func(tx *bolt.Tx) error {
		m := P(new(T))
		err := r.base.GetByIndexValue(tx, idx, value, m)
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

func (r *Registry[T, P]) DeleteEmptyBuckets(tx dbx.TransactionOrBucket, entityID string, bucketNames ...string) error {
	children := r.base.GetBucket(tx, r.childrenBucketName, entityID)
	if children == nil {
		return nil
	}

	var errs error
	for _, bucketName := range bucketNames {
		bucket := r.base.GetBucket(children, entityID)
		vals, err := r.base.GetIndexValues(bucket, bucketName)
		if err == nil && len(vals) > 0 {
			errs = errkit.Append(
				errs,
				errkit.Wrap(registry.ErrCannotDelete, fmt.Sprintf("%s has %s", r.entityName, bucketName)),
			)
			break // no sense to continue
		}

		errs = errkit.Append(errs, tx.DeleteBucket([]byte(entityID)))
	}

	return errs
}

func (r *Registry[T, P]) AddChild(childEntityBucketName, entityID, childID string) error {
	return r.db.Update(func(tx *bolt.Tx) error {
		m := P(new(T))
		err := r.base.Get(tx, entityID, m)
		if err != nil {
			return err
		}

		children := r.base.GetBucket(tx, r.childrenBucketName, m.GetID())
		if children == nil {
			return errkit.Wrap(registry.ErrNotFound, fmt.Sprintf("unknown %s id", r.entityName))
		}

		err = r.base.SaveIndexValue(children, childEntityBucketName, childID, childID)
		if err != nil {
			return err
		}

		return nil
	})
}

func (r *Registry[T, P]) GetChildren(childEntityBucketName, entityID string) ([]string, error) {
	var values map[string]string

	err := r.db.View(func(tx *bolt.Tx) error {
		m := P(new(T))
		err := r.base.Get(tx, entityID, m)
		if err != nil {
			return err
		}

		children := r.base.GetBucket(tx, r.childrenBucketName, m.GetID())
		if children == nil {
			return errkit.Wrap(registry.ErrNotFound, fmt.Sprintf("unknown %s id", r.entityName))
		}

		values, err = r.base.GetIndexValues(children, childEntityBucketName)
		if err != nil {
			return errkit.Wrap(err, fmt.Sprintf("failed to get %s", childEntityBucketName))
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	areas := make([]string, 0, len(values))

	for v := range values {
		areas = append(areas, v)
	}

	return areas, nil
}

func (r *Registry[T, P]) DeleteChild(childEntityBucketName, entityID, childID string) error {
	return r.db.Update(func(tx *bolt.Tx) error {
		m := P(new(T))
		err := r.base.Get(tx, entityID, m)
		if err != nil {
			return err
		}

		children := r.base.GetBucket(tx, r.childrenBucketName, m.GetID())
		if children == nil {
			return errkit.Wrap(registry.ErrNotFound, fmt.Sprintf("unknown %s id", r.entityName))
		}

		err = r.base.DeleteIndexValue(children, childEntityBucketName, childID)
		if err != nil {
			return err
		}

		return nil
	})
}
