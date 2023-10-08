package dbx

import (
	"os"
	"path/filepath"

	bolt "go.etcd.io/bbolt"

	"github.com/denisvmedia/inventario/internal/errkit"
)

type DB struct {
	dbPath string
	name   string

	db *bolt.DB
}

func NewDB(dbPath, name string) *DB {
	return &DB{
		dbPath: dbPath,
		name:   name,
	}
}

func (db *DB) Open() (result *bolt.DB, err error) {
	err = os.MkdirAll(db.dbPath, 0o700)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to create db directory")
	}

	dbPath := filepath.Join(db.dbPath, db.name)

	result, err = bolt.Open(dbPath, 0o600, nil)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to open db")
	}

	db.db = result

	return result, nil
}

func (db *DB) Close() error {
	if db.db == nil {
		panic("db is not opened")
	}

	return db.db.Close()
}

func (db *DB) Delete() error {
	if !db.exists() {
		return nil
	}

	err := db.db.Close()
	dbPath := filepath.Join(db.dbPath, db.name)
	errRemove := os.Remove(dbPath)

	return errkit.Append(err, errRemove)
}

func (db *DB) Exists() bool {
	return db.exists()
}

func (db *DB) exists() bool {
	info, err := os.Stat(filepath.Join(db.dbPath, db.name))
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}
