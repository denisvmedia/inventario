package debug

import (
	"encoding/json"
	"errors"
	"net/url"
	"runtime"

	"github.com/denisvmedia/inventario/internal/errkit"
)

// BaseInfo contains debug information about the application configuration
type BaseInfo struct {
	FileStorageDriver string `json:"file_storage_driver"`
	DatabaseDriver    string `json:"database_driver"`
	OperatingSystem   string `json:"operating_system"`
}

// Info contains debug information about the application configuration + possible error.
type Info struct {
	BaseInfo
	Error error `json:"error,omitempty"`
}

// InfoJSON is the JSON representation of Info.
type InfoJSON struct {
	BaseInfo
	Error json.RawMessage `json:"error,omitempty"`
}

func (info *Info) MarshalJSON() ([]byte, error) {
	if info == nil {
		return []byte("null"), nil
	}

	jsonData := InfoJSON{
		BaseInfo: info.BaseInfo,
	}

	// return json.Marshal(info)
	if info.Error != nil {
		jsonData.Error = errkit.ForceMarshalError(info.Error)
	}

	return json.Marshal(jsonData)
}

func NewInfo(dbDSN, uploadLocation string) *Info {
	info := &Info{}

	info.OperatingSystem = runtime.GOOS

	parsedDBDSN, err := url.Parse(dbDSN)
	if err != nil {
		info.Error = err
	} else {
		info.DatabaseDriver = parsedDBDSN.Scheme
	}
	parsedUploadLocation, err := url.Parse(uploadLocation)
	if err != nil {
		info.Error = errors.Join(info.Error, err)
	} else {
		info.FileStorageDriver = parsedUploadLocation.Scheme
	}

	return info
}
