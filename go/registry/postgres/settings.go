package postgres

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/go-extras/go-kit/must"
	"github.com/jmoiron/sqlx"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/postgres/store"
)

// SettingsRegistryFactory creates SettingsRegistry instances with proper context
type SettingsRegistryFactory struct {
	dbx        *sqlx.DB
	tableNames store.TableNames
}

// SettingsRegistry is a context-aware registry that can only be created through the factory
// Use the actual Setting model instead of a separate type
// This ensures consistency with the database schema
type SettingsRegistry struct {
	dbx        *sqlx.DB
	tableNames store.TableNames
	userID     string
	tenantID   string
	service    bool
}

var _ registry.SettingsRegistry = (*SettingsRegistry)(nil)
var _ registry.SettingsRegistryFactory = (*SettingsRegistryFactory)(nil)

func NewSettingsRegistry(dbx *sqlx.DB) *SettingsRegistryFactory {
	return NewSettingsRegistryWithTableNames(dbx, store.DefaultTableNames)
}

func NewSettingsRegistryWithTableNames(dbx *sqlx.DB, tableNames store.TableNames) *SettingsRegistryFactory {
	return &SettingsRegistryFactory{
		dbx:        dbx,
		tableNames: tableNames,
	}
}

// Factory methods implementing registry.SettingsRegistryFactory

func (f *SettingsRegistryFactory) MustCreateUserRegistry(ctx context.Context) registry.SettingsRegistry {
	return must.Must(f.CreateUserRegistry(ctx))
}

func (f *SettingsRegistryFactory) CreateUserRegistry(ctx context.Context) (registry.SettingsRegistry, error) {
	user, err := appctx.RequireUserFromContext(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to get user from context", err)
	}

	return &SettingsRegistry{
		dbx:        f.dbx,
		tableNames: f.tableNames,
		userID:     user.ID,
		tenantID:   user.TenantID,
		service:    false,
	}, nil
}

func (f *SettingsRegistryFactory) CreateServiceRegistry() registry.SettingsRegistry {
	return &SettingsRegistry{
		dbx:        f.dbx,
		tableNames: f.tableNames,
		userID:     "",
		tenantID:   "",
		service:    true,
	}
}

func (r *SettingsRegistry) Get(ctx context.Context) (models.SettingsObject, error) {
	var settings models.SettingsObject

	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		txReg := store.NewTxRegistry[models.Setting](tx, r.tableNames.Settings())

		for setting, err := range txReg.Scan(ctx) {
			if err != nil {
				return errxtrace.Wrap("failed to scan settings", err)
			}
			// Skip nil values - they shouldn't be set on the struct
			if setting.Value != nil {
				val, ok := setting.Value.([]byte)
				if !ok {
					return errors.New("failed to convert setting value to byte slice")
				}

				err := json.Unmarshal(val, &setting.Value)
				if err != nil {
					return errxtrace.Wrap("failed to unmarshal setting value", err)
				}

				if setting.Value == nil {
					continue
				}

				err = settings.Set(setting.Name, setting.Value)
				if err != nil {
					return errxtrace.Wrap("failed to set settings object value", err, errx.Attrs("setting_name", setting.Name))
				}
			}
		}
		return nil
	})
	if err != nil {
		return models.SettingsObject{}, errxtrace.Wrap("failed to get settings", err)
	}

	return settings, nil
}

func (r *SettingsRegistry) Save(ctx context.Context, settings models.SettingsObject) error {
	if r.service {
		return errors.New("service account cannot save settings")
	}

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
			fields := []store.FieldValue{
				store.Pair("name", settingName),
				store.Pair("user_id", r.userID),
				store.Pair("tenant_id", r.tenantID),
			}

			err := txReg.ScanOneByFields(ctx, fields, &sv)
			isNotFound := errors.Is(err, store.ErrNotFound)
			if err != nil && !isNotFound {
				return errxtrace.Wrap("failed to scan setting", err)
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
					return errxtrace.Wrap("failed to insert setting", err)
				}
			} else {
				// Update existing setting
				err = txReg.UpdateByField(ctx, store.Pair("name", settingName), sv)
				if err != nil {
					return errxtrace.Wrap("failed to update setting", err)
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
		return errxtrace.Wrap("invalid setting name", err)
	}

	reg := r.newSQLRegistry()
	err = reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		txReg := store.NewTxRegistry[models.Setting](tx, r.tableNames.Settings())

		var sv models.Setting
		err := txReg.ScanOneByField(ctx, store.Pair("name", settingName), &sv)
		isNotFound := errors.Is(err, store.ErrNotFound)
		if err != nil && !isNotFound {
			return errxtrace.Wrap("failed to scan setting", err)
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
				return errxtrace.Wrap("failed to insert setting", err)
			}
		} else {
			// Update existing setting
			err = txReg.UpdateByField(ctx, store.Pair("name", settingName), sv)
			if err != nil {
				return errxtrace.Wrap("failed to update setting", err)
			}
		}
		return nil
	})

	return err
}

func (r *SettingsRegistry) newSQLRegistry() *store.RLSRepository[models.Setting, *models.Setting] {
	if r.service {
		return store.NewServiceSQLRegistry[models.Setting](r.dbx, r.tableNames.Settings())
	}
	return store.NewUserAwareSQLRegistry[models.Setting](r.dbx, r.userID, r.tenantID, r.tableNames.Settings())
}
