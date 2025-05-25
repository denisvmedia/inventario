package commonsql

import (
	"context"
	"errors"

	"github.com/jmoiron/sqlx"

	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

var _ registry.SettingsRegistry = (*SettingsRegistry)(nil)

const settingsID = "settings"

type SettingsRegistry struct {
	dbx        *sqlx.DB
	tableNames TableNames
}

func NewSettingsRegistry(dbx *sqlx.DB) *SettingsRegistry {
	return NewSettingsRegistryWithTableNames(dbx, DefaultTableNames)
}

func NewSettingsRegistryWithTableNames(dbx *sqlx.DB, tableNames TableNames) *SettingsRegistry {
	return &SettingsRegistry{
		dbx:        dbx,
		tableNames: tableNames,
	}
}

func (r *SettingsRegistry) Get(ctx context.Context) (models.SettingsObject, error) {
	var settings models.SettingsObject
	for setting, err := range ScanEntities[models.Setting](ctx, r.dbx, r.tableNames.Images()) {
		if err != nil {
			return models.SettingsObject{}, errkit.Wrap(err, "failed to get settings")
		}
		err = settings.Set(setting.Name, setting.Value)
		if err != nil {
			return settings, errkit.Wrap(errkit.WithFields(err, "setting_name", setting.Name), "failed to set settings object value")
		}
	}

	return settings, nil
}

func (r *SettingsRegistry) Save(ctx context.Context, settings models.SettingsObject) error {
	// Begin a transaction (atomic operation)
	tx, err := r.dbx.Beginx()
	if err != nil {
		return errkit.Wrap(err, "failed to begin transaction")
	}
	defer func() {
		err = errors.Join(RollbackOrCommit(tx, err))
	}()

	settingsMap := settings.ToMap()

	for settingName, settingValue := range settingsMap {
		var sv models.Setting
		err := ScanEntityByField(ctx, tx, r.tableNames.Settings(), "name", settingName, &sv)
		if errors.Is(err, ErrNotFound) {
			sv.Name = settingName
			sv.Value = settingValue
			err = InsertEntity(ctx, tx, r.tableNames.Settings(), sv)
			if err != nil {
				return errkit.Wrap(err, "failed to insert setting")
			}
		} else {
			sv.Name = settingName
			sv.Value = settingValue
			err = UpdateEntityByField(ctx, tx, r.tableNames.Settings(), "name", settingName, sv)
			if err != nil {
				return errkit.Wrap(err, "failed to update setting")
			}
		}
	}

	return nil
}

func (r *SettingsRegistry) Patch(ctx context.Context, settingName string, settingValue any) error {
	// Begin a transaction (atomic operation)
	tx, err := r.dbx.Beginx()
	if err != nil {
		return errkit.Wrap(err, "failed to begin transaction")
	}
	defer func() {
		err = errors.Join(RollbackOrCommit(tx, err))
	}()

	var sv models.Setting
	err = ScanEntityByField(ctx, tx, r.tableNames.Settings(), "name", settingName, &sv)
	if errors.Is(err, ErrNotFound) {
		sv.Name = settingName
		sv.Value = settingValue
		err = InsertEntity(ctx, tx, r.tableNames.Settings(), sv)
		if err != nil {
			return errkit.Wrap(err, "failed to insert setting")
		}
	} else {
		sv.Name = settingName
		sv.Value = settingValue
		err = UpdateEntityByField(ctx, tx, r.tableNames.Settings(), "name", settingName, sv)
		if err != nil {
			return errkit.Wrap(err, "failed to update setting")
		}
	}

	return nil
}
