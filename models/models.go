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

type File struct {
	Path     string `json:"path"`
	Ext      string `json:"ext"`
	MIMEType string `json:"mime_type"`
}

func (i *File) Validate() error {
	fields := make([]*validation.FieldRules, 0)

	fields = append(fields,
		validation.Field(&i.Path, validation.Required),
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
