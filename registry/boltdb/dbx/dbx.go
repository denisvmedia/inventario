package dbx

import (
	"os"
	"path/filepath"

	bolt "go.etcd.io/bbolt"

	"github.com/denisvmedia/inventario/internal/errkit"
)

type DB struct {
	dbPath string
	db     *bolt.DB
}

func NewDB(dbPath string) *DB {
	return &DB{
		dbPath: dbPath,
	}
}

func (db *DB) Open() (result *bolt.DB, err error) {
	dbPath := filepath.Dir(db.dbPath)
	err = os.MkdirAll(db.dbPath, 0o700)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to create db directory")
	}

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
	errRemove := os.Remove(db.dbPath)

	return errkit.Append(err, errRemove)
}

func (db *DB) Exists() bool {
	return db.exists()
}

func (db *DB) exists() bool {
	info, err := os.Stat(db.dbPath)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}
