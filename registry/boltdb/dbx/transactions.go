package dbx

import (
	"encoding/json"
	"fmt"
	"reflect"

	bolt "go.etcd.io/bbolt"

	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/internal/typekit"
	"github.com/denisvmedia/inventario/registry"
)

type TransactionOrBucket interface {
	Bucket(name []byte) *bolt.Bucket
	CreateBucketIfNotExists(name []byte) (*bolt.Bucket, error)
	DeleteBucket(name []byte) error
}

type BaseRepository[T any, P registry.PIDable[T]] struct {
	bucket string
}

func NewBaseRepository[T any, P registry.PIDable[T]](bucketName string) *BaseRepository[T, P] {
	return &BaseRepository[T, P]{
		bucket: bucketName,
	}
}

func (s *BaseRepository[_, P]) GetAll(tx TransactionOrBucket, typ P) (results []P, err error) {
	if tx == nil || (reflect.ValueOf(tx).Kind() == reflect.Ptr && reflect.ValueOf(tx).IsNil()) {
		return nil, errkit.Wrap(ErrNotFound, "bucket or transaction does not exist")
	}

	b := tx.Bucket([]byte(s.bucket))
	if b == nil {
		return nil, errkit.Wrap(ErrNotFound, "bucket does not exist")
	}

	err = b.ForEach(func(_k, v []byte) error {
		elem := typekit.ZeroOfType(typ)
		err := json.Unmarshal(v, elem)
		if err != nil {
			return err
		}
		results = append(results, elem)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return results, nil
}

func (s *BaseRepository[_, _]) Exists(tx TransactionOrBucket, id string) bool {
	if tx == nil || (reflect.ValueOf(tx).Kind() == reflect.Ptr && reflect.ValueOf(tx).IsNil()) {
		return false
	}

	b := tx.Bucket([]byte(s.bucket))
	if b == nil {
		return false
	}
	v := b.Get([]byte(id))
	return v != nil
}

func (s *BaseRepository[_, _]) GetInterface(tx TransactionOrBucket, id string, m any) (err error) {
	if tx == nil || (reflect.ValueOf(tx).Kind() == reflect.Ptr && reflect.ValueOf(tx).IsNil()) {
		// return errkit.Wrap(ErrNotFound, "bucket or transaction does not exist")
		return ErrNotFound
	}

	b := tx.Bucket([]byte(s.bucket))
	if b == nil {
		// return errkit.Wrap(ErrNotFound, "bucket does not exist")
		return ErrNotFound
	}
	v := b.Get([]byte(id))
	if v == nil {
		return ErrNotFound
	}
	err = json.Unmarshal(v, m)
	if err != nil {
		return errkit.Wrap(ErrFailedToUnmarshalJSON, err.Error())
	}
	return nil
}

func (s *BaseRepository[_, P]) Get(tx TransactionOrBucket, id string, m P) (err error) {
	err = s.GetInterface(tx, id, m)
	if err != nil {
		return err
	}
	m.SetID(id)
	return nil
}

func (s *BaseRepository[_, P]) GetByIndexValue(tx TransactionOrBucket, idx, key string, m P) (err error) {
	if tx == nil || (reflect.ValueOf(tx).Kind() == reflect.Ptr && reflect.ValueOf(tx).IsNil()) {
		return errkit.Wrap(ErrNotFound, "bucket or transaction does not exist")
	}

	id, err := s.GetIndexValue(tx, idx, key)
	if err != nil {
		return err
	}
	err = s.Get(tx, id, m)
	if err != nil {
		return err
	}
	return nil
}

func (s *BaseRepository[_, _]) GetBucket(tx TransactionOrBucket, names ...string) (result *bolt.Bucket) {
	if tx == nil || (reflect.ValueOf(tx).Kind() == reflect.Ptr && reflect.ValueOf(tx).IsNil()) {
		return nil
	}

	if len(names) == 0 {
		return tx.Bucket([]byte(s.bucket))
	}

	result = tx.Bucket([]byte(names[0]))
	if result == nil || (reflect.ValueOf(result).Kind() == reflect.Ptr && reflect.ValueOf(result).IsNil()) {
		return nil
	}
	if len(names) == 1 {
		return result
	}

	for _, name := range names[1:] {
		result = result.Bucket([]byte(name))
		if result == nil || (reflect.ValueOf(result).Kind() == reflect.Ptr && reflect.ValueOf(result).IsNil()) {
			return nil
		}
	}

	return result
}

func (*BaseRepository[_, _]) GetOrCreateBucket(tx TransactionOrBucket, names ...string) (result *bolt.Bucket) {
	if len(names) == 0 {
		return nil
	}

	result, err := tx.CreateBucketIfNotExists([]byte(names[0]))
	if err != nil {
		panic(err)
	}
	if len(names) == 1 {
		return result
	}

	for _, name := range names[1:] {
		result, err = result.CreateBucketIfNotExists([]byte(name))
		if err != nil {
			panic(err)
		}
	}

	return result
}

func (*BaseRepository[_, _]) DeleteBucket(tx TransactionOrBucket, names ...string) {
	if len(names) == 0 {
		return
	}

	result := tx.Bucket([]byte(names[0]))
	if result == nil {
		panic("bucket does not exist: " + names[0])
	}
	if len(names) > 1 {
		for _, name := range names[1:] {
			err := result.DeleteBucket([]byte(name))
			if err != nil {
				panic(err)
			}
		}
	}

	err := tx.DeleteBucket([]byte(names[0]))
	if err != nil {
		panic(err)
	}
}

func (*BaseRepository[_, _]) GetIndexValues(tx TransactionOrBucket, idx string) (results map[string]string, err error) {
	if tx == nil || (reflect.ValueOf(tx).Kind() == reflect.Ptr && reflect.ValueOf(tx).IsNil()) {
		return nil, errkit.Wrap(ErrNotFound, "bucket or transaction does not exist")
	}

	b := tx.Bucket([]byte(idx))
	if b == nil || (reflect.ValueOf(b).Kind() == reflect.Ptr && reflect.ValueOf(b).IsNil()) {
		return nil, errkit.Wrap(ErrNotFound, "bucket does not exist")
	}

	results = make(map[string]string)
	err = b.ForEach(func(k, v []byte) error {
		results[string(k)] = string(v)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return results, nil
}

func (s *BaseRepository[_, _]) GetIndexValue(tx TransactionOrBucket, idx, key string) (string, error) {
	v, err := s.GetIndexValueBytes(tx, idx, []byte(key))
	return string(v), err
}

func (*BaseRepository[_, _]) GetIndexValueBytes(tx TransactionOrBucket, idx string, key []byte) (val []byte, err error) {
	if tx == nil || (reflect.ValueOf(tx).Kind() == reflect.Ptr && reflect.ValueOf(tx).IsNil()) {
		return nil, errkit.Wrap(ErrNotFound, "bucket or transaction does not exist")
	}

	b := tx.Bucket([]byte(idx))
	if b == nil || (reflect.ValueOf(b).Kind() == reflect.Ptr && reflect.ValueOf(b).IsNil()) {
		return nil, errkit.Wrap(ErrNotFound, "bucket does not exist")
	}
	v := b.Get(key)
	if v == nil {
		return nil, ErrNotFound
	}
	return v, nil
}

func (s *BaseRepository[_, _]) SetInterface(tx TransactionOrBucket, id string, m any) error {
	if tx == nil || (reflect.ValueOf(tx).Kind() == reflect.Ptr && reflect.ValueOf(tx).IsNil()) {
		return errkit.Wrap(ErrNotFound, "bucket or transaction does not exist")
	}

	b, err := tx.CreateBucketIfNotExists([]byte(s.bucket))
	if err != nil {
		panic(err)
	}

	buf, err := json.Marshal(m)
	if err != nil {
		return err
	}

	// Persist bytes to bucket.
	err = b.Put([]byte(id), buf)
	if err != nil {
		return err
	}

	return nil
}

func (s *BaseRepository[_, P]) Save(tx TransactionOrBucket, entity P) error {
	if tx == nil || (reflect.ValueOf(tx).Kind() == reflect.Ptr && reflect.ValueOf(tx).IsNil()) {
		return errkit.Wrap(ErrNotFound, "bucket or transaction does not exist")
	}

	b, err := tx.CreateBucketIfNotExists([]byte(s.bucket))
	if err != nil {
		panic(err)
	}

	if entity.GetID() == "" {
		id, err := b.NextSequence()
		if err != nil {
			panic(err)
		}
		entity.SetID(fmt.Sprint(id))
	}

	// Marshal data into bytes.
	buf, err := json.Marshal(entity)
	if err != nil {
		return err
	}

	// Persist bytes to bucket.
	err = b.Put([]byte(entity.GetID()), buf)
	if err != nil {
		return err
	}

	return nil
}

func (s *BaseRepository[_, _]) SaveIndexValue(tx TransactionOrBucket, idx, key, val string) error {
	return s.SaveIndexValueBytes(tx, idx, []byte(key), []byte(val))
}

func (*BaseRepository[_, _]) SaveIndexValueBytes(tx TransactionOrBucket, idx string, key, val []byte) error {
	if tx == nil || (reflect.ValueOf(tx).Kind() == reflect.Ptr && reflect.ValueOf(tx).IsNil()) {
		return errkit.Wrap(ErrNotFound, "bucket or transaction does not exist")
	}

	b, err := tx.CreateBucketIfNotExists([]byte(idx))
	if err != nil {
		panic(err)
	}

	// Persist bytes to bucket.
	err = b.Put(key, val)
	if err != nil {
		return err
	}

	return nil
}

func (s *BaseRepository[_, _]) Delete(tx TransactionOrBucket, id string) error {
	if tx == nil || (reflect.ValueOf(tx).Kind() == reflect.Ptr && reflect.ValueOf(tx).IsNil()) {
		return errkit.Wrap(ErrNotFound, "bucket or transaction does not exist")
	}

	b, err := tx.CreateBucketIfNotExists([]byte(s.bucket))
	if err != nil {
		panic(err)
	}

	// Persist bytes to bucket.
	err = b.Delete([]byte(id))
	if err != nil {
		return err
	}

	return nil
}

func (s *BaseRepository[_, _]) DeleteByIndexValue(tx TransactionOrBucket, idx, key string) (err error) {
	if tx == nil || (reflect.ValueOf(tx).Kind() == reflect.Ptr && reflect.ValueOf(tx).IsNil()) {
		return errkit.Wrap(ErrNotFound, "bucket or transaction does not exist")
	}

	id, err := s.GetIndexValue(tx, idx, key)
	if err != nil {
		return err
	}
	err = s.Delete(tx, id)
	if err != nil {
		return err
	}
	return nil
}

func (*BaseRepository[_, _]) DeleteIndexValue(tx TransactionOrBucket, idx, name string) error {
	if tx == nil || (reflect.ValueOf(tx).Kind() == reflect.Ptr && reflect.ValueOf(tx).IsNil()) {
		return errkit.Wrap(ErrNotFound, "bucket or transaction does not exist")
	}

	b, err := tx.CreateBucketIfNotExists([]byte(idx))
	if err != nil {
		panic(err)
	}

	// Persist bytes to users bucket.
	err = b.Delete([]byte(name))
	if err != nil {
		return err
	}

	return nil
}

func (s *BaseRepository[_, _]) Count(tx TransactionOrBucket, names ...string) (int, error) {
	var count int

	b := s.GetBucket(tx, names...)

	c := b.Cursor()
	for k, v := c.First(); k != nil; k, v = c.Next() {
		if v != nil { // Ensure the value exists (it should, but just to be safe)
			count++
		}
	}

	return count, nil
}
