package boltdb

import (
	"context"
	"errors"
	"fmt"

	bolt "go.etcd.io/bbolt"

	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/boltdb/dbx"
)

type HookFn[T any, P registry.PIDable[T]] func(dbx.TransactionOrBucket, P) error

type Registry[T any, P registry.PIDable[T]] struct {
	db                 *bolt.DB
	base               *dbx.BaseRepository[T, P]
	entityName         string
	childrenBucketName string
}

func NewRegistry[T any, P registry.PIDable[T]](
	db *bolt.DB,
	base *dbx.BaseRepository[T, P],
	entityName,
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
			return errkit.Wrap(err, "failed to create entity")
		}

		result.SetID("") // ignore the id
		err = r.base.Save(tx, result)
		if err != nil {
			return errkit.Wrap(err, "failed to create entity")
		}

		err = after(tx, result)
		if err != nil {
			return errkit.Wrap(err, "failed to create entity")
		}

		return nil
	})
	if err != nil {
		return nil, errkit.Wrap(err, "failed to create entity")
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
		return nil, errkit.Wrap(err, "failed to get entity")
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
		return nil, errkit.Wrap(err, "failed to get entity")
	}

	return result, nil
}

func (r *Registry[T, P]) List() (results []P, err error) {
	err = r.db.View(func(tx *bolt.Tx) error {
		val, err := r.base.GetAll(tx, P(new(T)))
		if err != nil {
			return errkit.Wrap(err, "failed to obtain entities")
		}
		results = val
		return nil
	})

	if errors.Is(err, registry.ErrNotFound) {
		return nil, nil
	}

	if err != nil {
		return nil, errkit.Wrap(err, "failed to list entities")
	}

	return results, nil
}

func (r *Registry[T, P]) Update(m T, before, after HookFn[T, P]) (result P, err error) {
	result = &m
	err = r.db.Update(func(tx *bolt.Tx) error {
		old := P(new(T))

		err := r.base.Get(tx, result.GetID(), old)
		if err != nil {
			return errkit.Wrap(err, "failed to update entity")
		}

		err = before(tx, result)
		if err != nil {
			return errkit.Wrap(err, "failed to update entity")
		}

		err = r.base.Save(tx, result)
		if err != nil {
			return errkit.Wrap(err, "failed to update entity")
		}

		err = after(tx, result)
		if err != nil {
			return errkit.Wrap(err, "failed to update entity")
		}

		return nil
	})
	if err != nil {
		return nil, errkit.Wrap(err, "failed to update entity")
	}

	return result, nil
}

func (r *Registry[T, P]) Count() (int, error) {
	var cnt int

	err := r.db.View(func(tx *bolt.Tx) error {
		var err error
		cnt, err = r.base.Count(tx)
		if err != nil {
			return errkit.Wrap(err, "failed to count entities")
		}

		return nil
	})
	if err != nil {
		return 0, errkit.Wrap(err, "failed to count entities")
	}

	return cnt, nil
}

func (r *Registry[T, P]) Delete(id string, before, after HookFn[T, P]) error {
	return r.db.Update(func(tx *bolt.Tx) error {
		m := P(new(T))
		err := r.base.Get(tx, id, m)
		if err != nil {
			return errkit.Wrap(err, "failed to delete entity")
		}

		err = before(tx, m)
		if err != nil {
			return errkit.Wrap(err, "failed to delete entity")
		}

		err = r.base.Delete(tx, id)
		if err != nil {
			return errkit.Wrap(err, "failed to delete entity")
		}

		err = after(tx, m)
		if err != nil {
			return errkit.Wrap(err, "failed to delete entity")
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
		bucket := r.base.GetBucket(children, bucketName)
		vals, err := r.base.GetIndexValues(bucket, bucketName)
		if err == nil && len(vals) > 0 {
			errs = errkit.Append(
				errs,
				errkit.Wrap(registry.ErrCannotDelete, fmt.Sprintf("%s has %s", r.entityName, bucketName)),
			)
			return errkit.WithStack(errs) // Return immediately if we find a non-empty bucket
		}
	}

	// Only delete the bucket if all child buckets are empty
	return nil
}

func (r *Registry[T, P]) AddChild(childEntityBucketName, entityID, childID string) error {
	return r.db.Update(func(tx *bolt.Tx) error {
		m := P(new(T))
		err := r.base.Get(tx, entityID, m)
		if err != nil {
			return errkit.Wrap(err, "failed to get entity")
		}

		children := r.base.GetBucket(tx, r.childrenBucketName, m.GetID())
		if children == nil {
			return errkit.Wrap(registry.ErrNotFound, fmt.Sprintf("unknown %s id", r.entityName))
		}

		err = r.base.SaveIndexValue(children, childEntityBucketName, childID, childID)
		if err != nil {
			return errkit.Wrap(err, fmt.Sprintf("failed to add %s", childEntityBucketName))
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
			return errkit.Wrap(err, "failed to get entity")
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
		return nil, errkit.Wrap(err, fmt.Sprintf("failed to get %s", childEntityBucketName))
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
			return errkit.Wrap(err, "failed to get entity")
		}

		children := r.base.GetBucket(tx, r.childrenBucketName, m.GetID())
		if children == nil {
			return errkit.Wrap(registry.ErrNotFound, fmt.Sprintf("unknown %s id", r.entityName))
		}

		err = r.base.DeleteIndexValue(children, childEntityBucketName, childID)
		if err != nil {
			return errkit.Wrap(err, fmt.Sprintf("failed to delete %s", childEntityBucketName))
		}

		return nil
	})
}

// User-aware methods that filter by user_id

// SetUserContext is a no-op for BoltDB as it doesn't use database-level RLS
func (r *Registry[T, P]) SetUserContext(ctx context.Context, userID string) error {
	// BoltDB doesn't support database-level RLS, so this is a no-op
	// User filtering is done at the application level
	return nil
}

// WithUserContext executes a function with user context (no-op for BoltDB)
func (r *Registry[T, P]) WithUserContext(ctx context.Context, userID string, fn func(context.Context) error) error {
	// BoltDB doesn't support database-level RLS, so just execute the function
	return fn(ctx)
}

// CreateWithUser creates an entity with user context
func (r *Registry[T, P]) CreateWithUser(ctx context.Context, m T) (P, error) {
	// Extract user ID from context
	userID := registry.UserIDFromContext(ctx)
	if userID == "" {
		return nil, errkit.WithStack(registry.ErrUserContextRequired)
	}

	result := P(&m)

	// Set user_id on the entity if it's UserAware
	if userAware, ok := any(result).(models.UserAware); ok {
		userAware.SetUserID(userID)
	}

	// Use the regular Create method with empty hooks
	return r.Create(m, func(tx dbx.TransactionOrBucket, p P) error { return nil }, func(tx dbx.TransactionOrBucket, p P) error { return nil })
}

// GetWithUser gets an entity with user context and validates ownership
func (r *Registry[T, P]) GetWithUser(ctx context.Context, id string) (P, error) {
	// Extract user ID from context
	userID := registry.UserIDFromContext(ctx)
	if userID == "" {
		return nil, errkit.WithStack(registry.ErrUserContextRequired)
	}

	// Get the entity
	result, err := r.Get(id)
	if err != nil {
		return nil, err
	}

	// Check if the entity belongs to the user
	if userAware, ok := any(result).(models.UserAware); ok {
		if userAware.GetUserID() != userID {
			return nil, errkit.WithStack(registry.ErrNotFound)
		}
	}

	return result, nil
}

// ListWithUser lists entities with user context filtering
func (r *Registry[T, P]) ListWithUser(ctx context.Context) ([]P, error) {
	// Extract user ID from context
	userID := registry.UserIDFromContext(ctx)
	if userID == "" {
		return nil, errkit.WithStack(registry.ErrUserContextRequired)
	}

	// Get all entities
	allResults, err := r.List()
	if err != nil {
		return nil, err
	}

	// Filter by user_id
	var filteredResults []P
	for _, result := range allResults {
		if userAware, ok := any(result).(models.UserAware); ok {
			if userAware.GetUserID() == userID {
				filteredResults = append(filteredResults, result)
			}
		} else {
			// If entity is not UserAware, include it (for backward compatibility)
			filteredResults = append(filteredResults, result)
		}
	}

	return filteredResults, nil
}

// UpdateWithUser updates an entity with user context
func (r *Registry[T, P]) UpdateWithUser(ctx context.Context, m T) (P, error) {
	// Extract user ID from context
	userID := registry.UserIDFromContext(ctx)
	if userID == "" {
		return nil, errkit.WithStack(registry.ErrUserContextRequired)
	}

	result := P(&m)

	// Set user_id on the entity if it's UserAware
	if userAware, ok := any(result).(models.UserAware); ok {
		userAware.SetUserID(userID)
	}

	// Validate ownership before update
	existing, err := r.GetWithUser(ctx, result.GetID())
	if err != nil {
		return nil, err
	}
	_ = existing // We just need to validate ownership

	// Use the regular Update method with empty hooks
	return r.Update(m, func(tx dbx.TransactionOrBucket, p P) error { return nil }, func(tx dbx.TransactionOrBucket, p P) error { return nil })
}

// DeleteWithUser deletes an entity with user context
func (r *Registry[T, P]) DeleteWithUser(ctx context.Context, id string) error {
	// Extract user ID from context
	userID := registry.UserIDFromContext(ctx)
	if userID == "" {
		return errkit.WithStack(registry.ErrUserContextRequired)
	}

	// Validate ownership before delete
	_, err := r.GetWithUser(ctx, id)
	if err != nil {
		return err
	}

	// Use the regular Delete method with empty hooks
	return r.Delete(id, func(tx dbx.TransactionOrBucket, p P) error { return nil }, func(tx dbx.TransactionOrBucket, p P) error { return nil })
}

// CountWithUser counts entities with user context filtering
func (r *Registry[T, P]) CountWithUser(ctx context.Context) (int, error) {
	// Extract user ID from context
	userID := registry.UserIDFromContext(ctx)
	if userID == "" {
		return 0, errkit.WithStack(registry.ErrUserContextRequired)
	}

	// Get filtered list and return count
	filteredResults, err := r.ListWithUser(ctx)
	if err != nil {
		return 0, err
	}

	return len(filteredResults), nil
}
