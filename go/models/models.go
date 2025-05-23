package models

import (
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
	Path string `json:"path"`

	// OriginalPath is the original filename as uploaded by the user.
	// Example: "invoice.pdf"
	OriginalPath string `json:"original_path"`

	// Ext is the file extension including the dot.
	// Example: ".pdf"
	Ext string `json:"ext"`

	// MIMEType is the MIME type of the file.
	// Example: "application/pdf"
	MIMEType string `json:"mime_type"`
}

func (i *File) Validate() error {
	fields := make([]*validation.FieldRules, 0)

	fields = append(fields,
		validation.Field(&i.Path, validation.Required),
		validation.Field(&i.OriginalPath, validation.Required),
		validation.Field(&i.Ext, validation.Required),
		validation.Field(&i.MIMEType, validation.Required),
	)

	return validation.ValidateStruct(i, fields...)
}

var (
	_ IDable = (*EntityID)(nil)
)

type EntityID struct {
	ID string `json:"id"`
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
