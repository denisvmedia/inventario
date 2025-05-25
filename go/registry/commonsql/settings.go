package commonsql

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jmoiron/sqlx"

	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

var _ registry.SettingsRegistry = (*SettingsRegistry)(nil)

type settingType struct {
	Name  string          `db:"name"`
	Value json.RawMessage `db:"value"`
}

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
	for setting, err := range ScanEntities[settingType](ctx, r.dbx, r.tableNames.Settings()) {
		if err != nil {
			return models.SettingsObject{}, errkit.Wrap(err, "failed to get settings")
		}
		var value any
		err = json.Unmarshal(setting.Value, &value)
		if err != nil {
			return models.SettingsObject{}, errkit.Wrap(err, "failed to unmarshal setting value")
		}
		err = settings.Set(setting.Name, value)
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
		err = errors.Join(err, RollbackOrCommit(tx, err))
	}()

	settingsMap := settings.ToMap()

	for settingName, settingValue := range settingsMap {
		var sv settingType
		err := ScanEntityByField(ctx, tx, r.tableNames.Settings(), "name", settingName, &sv)
		if errors.Is(err, ErrNotFound) {
			sv.Name = settingName
			sv.Value, err = json.Marshal(settingValue)
			if err != nil {
				return errkit.Wrap(err, "failed to marshal setting value")
			}
			err = InsertEntity(ctx, tx, r.tableNames.Settings(), sv)
			if err != nil {
				return errkit.Wrap(err, "failed to insert setting")
			}
		} else {
			sv.Name = settingName
			sv.Value, err = json.Marshal(settingValue)
			if err != nil {
				return errkit.Wrap(err, "failed to marshal setting value")
			}
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
		err = errors.Join(err, RollbackOrCommit(tx, err))
	}()

	var sv settingType
	err = ScanEntityByField(ctx, tx, r.tableNames.Settings(), "name", settingName, &sv)
	if errors.Is(err, ErrNotFound) {
		sv.Name = settingName
		sv.Value, err = json.Marshal(settingValue)
		if err != nil {
			return errkit.Wrap(err, "failed to marshal setting value")
		}
		err = InsertEntity(ctx, tx, r.tableNames.Settings(), sv)
		if err != nil {
			return errkit.Wrap(err, "failed to insert setting")
		}
	} else {
		sv.Name = settingName
		sv.Value, err = json.Marshal(settingValue)
		if err != nil {
			return errkit.Wrap(err, "failed to marshal setting value")
		}
		err = UpdateEntityByField(ctx, tx, r.tableNames.Settings(), "name", settingName, sv)
		if err != nil {
			return errkit.Wrap(err, "failed to update setting")
		}
	}

	return nil
}
