package models

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"fmt"

	"github.com/jellydator/validation"
)

type IDable interface {
	GetID() string
	SetID(string)
}

var (
	_ validation.Validatable = (*File)(nil)
)

// File represents a file in the system with its metadata.
// Example:
//
//	{
//	  "path": "invoice-2023",           // Just the filename without extension (editable by user)
//	  "original_path": "invoice.pdf",   // Original filename as uploaded
//	  "ext": ".pdf",                   // File extension including the dot
//	  "mime_type": "application/pdf"    // MIME type of the file
//	}
type File struct {
	// Path is the filename without extension. This is the only field that can be modified by the user.
	// Example: "invoice-2023"
	Path string `json:"path" db:"path"`

	// OriginalPath is the original filename as uploaded by the user.
	// Example: "invoice.pdf"
	OriginalPath string `json:"original_path" db:"original_path"`

	// Ext is the file extension including the dot.
	// Example: ".pdf"
	Ext string `json:"ext" db:"ext"`

	// MIMEType is the MIME type of the file.
	// Example: "application/pdf"
	MIMEType string `json:"mime_type" db:"mime_type"`
}

func (i *File) Validate() error {
	return ErrMustUseValidateWithContext
}

func (i *File) ValidateWithContext(ctx context.Context) error {
	fields := make([]*validation.FieldRules, 0)

	fields = append(fields,
		validation.Field(&i.Path, validation.Required),
		validation.Field(&i.OriginalPath, validation.Required),
		validation.Field(&i.Ext, validation.Required),
		validation.Field(&i.MIMEType, validation.Required),
	)

	return validation.ValidateStructWithContext(ctx, i, fields...)
}

var (
	_ IDable = (*EntityID)(nil)
)

type EntityID struct {
	ID string `json:"id" db:"id"`
}

func (i *EntityID) GetID() string {
	return i.ID
}

func (i *EntityID) SetID(id string) {
	i.ID = id
}

func WithID[T IDable](id string, i T) T {
	i.SetID(id)
	return i
}

type ValuerSlice[T any] []T

func (s *ValuerSlice[T]) Scan(src any) error {
	if src == nil {
		*s = nil
		return nil
	}
	bytes, ok := src.([]byte)
	if !ok {
		return fmt.Errorf("cannot scan type %T into StringSlice", src)
	}
	return json.Unmarshal(bytes, s)
}

func (s ValuerSlice[T]) Value() (driver.Value, error) {
	if s == nil {
		return nil, nil
	}
	return json.Marshal(s)
}
