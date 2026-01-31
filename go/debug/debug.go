package debug

import (
	"encoding/json"
	"errors"
	"net/url"
	"runtime"

	errxjson "github.com/go-extras/errx/json"
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

func (inf *Info) MarshalJSON() ([]byte, error) {
	if inf == nil {
		return []byte("null"), nil
	}

	jsonData := InfoJSON{
		BaseInfo: inf.BaseInfo,
	}

	// return json.Marshal(info)
	if inf.Error != nil {
		errorBytes, err := errxjson.Marshal(inf.Error)
		if err != nil {
			// If we can't marshal the error, use a simple error message
			jsonData.Error = json.RawMessage(`"error marshaling failed"`)
		} else {
			jsonData.Error = errorBytes
		}
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
