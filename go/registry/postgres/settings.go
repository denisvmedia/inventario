package postgres

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/go-extras/go-kit/must"
	"github.com/jmoiron/sqlx"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/postgres/store"
)

var _ registry.SettingsRegistry = (*SettingsRegistry)(nil)

// Use the actual Setting model instead of a separate type
// This ensures consistency with the database schema

type SettingsRegistry struct {
	dbx        *sqlx.DB
	tableNames store.TableNames
	userID     string
	tenantID   string
}

func NewSettingsRegistry(dbx *sqlx.DB) *SettingsRegistry {
	return NewSettingsRegistryWithTableNames(dbx, store.DefaultTableNames)
}

func NewSettingsRegistryWithTableNames(dbx *sqlx.DB, tableNames store.TableNames) *SettingsRegistry {
	return &SettingsRegistry{
		dbx:        dbx,
		tableNames: tableNames,
	}
}

func (r *SettingsRegistry) WithCurrentUser(ctx context.Context) (registry.SettingsRegistry, error) {
	tmp := *r

	user, err := appctx.RequireUserFromContext(ctx)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to get user from context")
	}
	tmp.userID = user.ID
	tmp.tenantID = user.TenantID
	return &tmp, nil
}

func (r *SettingsRegistry) Get(ctx context.Context) (models.SettingsObject, error) {
	var settings models.SettingsObject

	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		txReg := store.NewTxRegistry[models.Setting](tx, r.tableNames.Settings())

		for setting, err := range txReg.Scan(ctx) {
			if err != nil {
				return errkit.Wrap(err, "failed to scan settings")
			}
			// Skip nil values - they shouldn't be set on the struct
			if setting.Value != nil {
				val, ok := setting.Value.([]byte)
				if !ok {
					return errors.New("failed to convert setting value to byte slice")
				}

				err := json.Unmarshal(val, &setting.Value)
				if err != nil {
					return errkit.Wrap(err, "failed to unmarshal setting value")
				}

				if setting.Value == nil {
					continue
				}

				err = settings.Set(setting.Name, setting.Value)
				if err != nil {
					return errkit.Wrap(errkit.WithFields(err, "setting_name", setting.Name), "failed to set settings object value")
				}
			}
		}
		return nil
	})
	if err != nil {
		return models.SettingsObject{}, errkit.Wrap(err, "failed to get settings")
	}

	return settings, nil
}

func (r *SettingsRegistry) Save(ctx context.Context, settings models.SettingsObject) error {
	settingsMap := settings.ToMap()

	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		txReg := store.NewTxRegistry[models.Setting](tx, r.tableNames.Settings())

		for settingName, settingValue := range settingsMap {
			// Skip nil values - we don't want to store them in the database
			if settingValue == nil {
				continue
			}

			var sv models.Setting
			err := txReg.ScanOneByField(ctx, store.Pair("name", settingName), &sv)
			isNotFound := errors.Is(err, store.ErrNotFound)
			if err != nil && !isNotFound {
				return errkit.Wrap(err, "failed to scan setting")
			}

			// Prepare the setting value
			if isNotFound {
				// Create new setting with ID
				sv.SetID(generateID())
				sv.SetUserID(r.userID)
				sv.SetTenantID(r.tenantID)
			}
			sv.Name = settingName
			sv.Value = must.Must(json.Marshal(settingValue))

			if isNotFound {
				// Insert new setting
				err = txReg.Insert(ctx, sv)
				if err != nil {
					return errkit.Wrap(err, "failed to insert setting")
				}
			} else {
				// Update existing setting
				err = txReg.UpdateByField(ctx, store.Pair("name", settingName), sv)
				if err != nil {
					return errkit.Wrap(err, "failed to update setting")
				}
			}
		}
		return nil
	})

	return err
}

func (r *SettingsRegistry) Patch(ctx context.Context, settingName string, settingValue any) error {
	// Validate the setting name by trying to set it on a temporary settings object
	var tempSettings models.SettingsObject
	err := tempSettings.Set(settingName, settingValue)
	if err != nil {
		return errkit.Wrap(err, "invalid setting name")
	}

	reg := r.newSQLRegistry()
	err = reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		txReg := store.NewTxRegistry[models.Setting](tx, r.tableNames.Settings())

		var sv models.Setting
		err := txReg.ScanOneByField(ctx, store.Pair("name", settingName), &sv)
		isNotFound := errors.Is(err, store.ErrNotFound)
		if err != nil && !isNotFound {
			return errkit.Wrap(err, "failed to scan setting")
		}

		// Prepare the setting value
		if isNotFound {
			// Create new setting with ID
			sv.SetID(generateID())
			sv.SetUserID(r.userID)
			sv.SetTenantID(r.tenantID)
		}
		sv.Name = settingName
		sv.Value = must.Must(json.Marshal(settingValue))

		if isNotFound {
			// Insert new setting
			err = txReg.Insert(ctx, sv)
			if err != nil {
				return errkit.Wrap(err, "failed to insert setting")
			}
		} else {
			// Update existing setting
			err = txReg.UpdateByField(ctx, store.Pair("name", settingName), sv)
			if err != nil {
				return errkit.Wrap(err, "failed to update setting")
			}
		}
		return nil
	})

	return err
}

func (r *SettingsRegistry) newSQLRegistry() *store.RLSRepository[models.Setting] {
	return store.NewUserAwareSQLRegistry[models.Setting](r.dbx, r.userID, r.tableNames.Settings())
}
