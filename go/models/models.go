package models

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strings"
	"time"

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

func (*File) Validate() error {
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

// FileType represents the type/category of a file
type FileType string

const (
	FileTypeImage    FileType = "image"
	FileTypeDocument FileType = "document"
	FileTypeVideo    FileType = "video"
	FileTypeAudio    FileType = "audio"
	FileTypeArchive  FileType = "archive"
	FileTypeOther    FileType = "other"
)

// FileTypeFromMIME determines the file type based on MIME type
func FileTypeFromMIME(mimeType string) FileType {
	switch {
	case strings.HasPrefix(mimeType, "image/"):
		return FileTypeImage
	case strings.HasPrefix(mimeType, "video/"):
		return FileTypeVideo
	case strings.HasPrefix(mimeType, "audio/"):
		return FileTypeAudio
	case mimeType == "application/zip" || mimeType == "application/x-zip-compressed":
		return FileTypeArchive
	case mimeType == "application/pdf" ||
		mimeType == "text/plain" ||
		mimeType == "text/csv" ||
		mimeType == "application/json" ||
		strings.HasPrefix(mimeType, "application/vnd.ms-") ||
		strings.HasPrefix(mimeType, "application/vnd.openxmlformats-") ||
		mimeType == "application/msword":
		return FileTypeDocument
	default:
		return FileTypeOther
	}
}

// FileEntity represents a file entity in the system
type FileEntity struct {
	EntityID

	// Title is the user-defined title for the file
	Title string `json:"title" db:"title"`

	// Description is an optional description of the file
	Description string `json:"description" db:"description"`

	// Type represents the category of the file (image, document, etc.)
	Type FileType `json:"type" db:"type"`

	// Tags are optional tags for categorization and search
	Tags []string `json:"tags" db:"tags"`

	// CreatedAt is when the file was created
	CreatedAt time.Time `json:"created_at" db:"created_at"`

	// UpdatedAt is when the file was last updated
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`

	// File contains the actual file metadata
	*File
}

func (*FileEntity) Validate() error {
	return ErrMustUseValidateWithContext
}

func (fe *FileEntity) ValidateWithContext(ctx context.Context) error {
	fields := make([]*validation.FieldRules, 0)

	fields = append(fields,
		validation.Field(&fe.Title, validation.Required, validation.Length(1, 255)),
		validation.Field(&fe.Description, validation.Length(0, 1000)),
		validation.Field(&fe.Type, validation.Required, validation.In(
			FileTypeImage, FileTypeDocument, FileTypeVideo,
			FileTypeAudio, FileTypeArchive, FileTypeOther,
		)),
		validation.Field(&fe.File, validation.Required),
	)

	return validation.ValidateStructWithContext(ctx, fe, fields...)
}

var (
	_ IDable                 = (*EntityID)(nil)
	_ validation.Validatable = (*FileEntity)(nil)
)

type EntityID struct {
	ID string `json:"id" db:"id" userinput:"false"`
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
