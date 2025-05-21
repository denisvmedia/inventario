package boltdb

import (
	"encoding/json"

	bolt "go.etcd.io/bbolt"

	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/internal/typekit"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

const (
	bucketNameSettings = "settings"
	settingsKey        = "settings"
)

var _ registry.SettingsRegistry = (*SettingsRegistry)(nil)

type SettingsRegistry struct {
	db *bolt.DB
}

func NewSettingsRegistry(db *bolt.DB) *SettingsRegistry {
	return &SettingsRegistry{
		db: db,
	}
}

func (r *SettingsRegistry) Get() (models.SettingsObject, error) {
	var settings models.SettingsObject

	err := r.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(bucketNameSettings))
		if bucket == nil {
			// If the bucket doesn't exist, return empty settings
			return nil
		}

		data := bucket.Get([]byte(settingsKey))
		if data == nil {
			// If the settings don't exist, return empty settings
			return nil
		}

		return json.Unmarshal(data, &settings)
	})

	if err != nil {
		return models.SettingsObject{}, errkit.Wrap(err, "failed to get settings")
	}

	return settings, nil
}

func (r *SettingsRegistry) Save(settings models.SettingsObject) error {
	return r.db.Update(func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte(bucketNameSettings))
		if err != nil {
			return errkit.Wrap(err, "failed to create settings bucket")
		}

		data, err := json.Marshal(settings)
		if err != nil {
			return errkit.Wrap(err, "failed to marshal settings")
		}

		err = bucket.Put([]byte(settingsKey), data)
		if err != nil {
			return errkit.Wrap(err, "failed to save settings")
		}

		return nil
	})
}

func (r *SettingsRegistry) Patch(configfield string, value any) error {
	return r.db.Update(func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte(bucketNameSettings))
		if err != nil {
			return errkit.Wrap(err, "failed to create settings bucket")
		}

		data := bucket.Get([]byte(settingsKey))
		var settings models.SettingsObject
		if data != nil {
			err = json.Unmarshal(data, &settings)
			if err != nil {
				return errkit.Wrap(err, "failed to unmarshal settings")
			}
		}

		err = typekit.SetFieldByConfigfieldTag(&settings, configfield, value)
		if err != nil {
			return errkit.Wrap(err, "failed to patch settings")
		}

		data, err = json.Marshal(settings)
		if err != nil {
			return errkit.Wrap(err, "failed to marshal settings")
		}

		err = bucket.Put([]byte(settingsKey), data)
		if err != nil {
			return errkit.Wrap(err, "failed to save settings")
		}

		return nil
	})
}
