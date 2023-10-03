package models

import (
	"encoding/json"
	"net/url"

	"github.com/jellydator/validation"
)

func URLParse(s string) (*URL, error) {
	u, err := url.Parse(s)
	if err != nil {
		return nil, err
	}
	return (*URL)(u), nil
}

var (
	_ validation.Validatable = (*URL)(nil)
	_ json.Marshaler         = (*URL)(nil)
	_ json.Unmarshaler       = (*URL)(nil)
)

type URL url.URL

func (u *URL) String() string {
	return (*url.URL)(u).String()
}

func (u *URL) Validate() error {
	fields := make([]*validation.FieldRules, 0)

	fields = append(fields,
		validation.Field(&u.Host, validation.Required),
		validation.Field(&u.Scheme, validation.Required, validation.In("http", "https")),
	)

	return validation.ValidateStruct(u, fields...)
}

func (u *URL) MarshalJSON() ([]byte, error) {
	tmp := (*url.URL)(u)
	return json.Marshal(tmp.String())
}

func (u *URL) UnmarshalJSON(data []byte) error {
	var tmp string
	err := json.Unmarshal(data, &tmp)
	if err != nil {
		return err
	}

	v, err := url.Parse(tmp)
	if err != nil {
		return err
	}

	*u = URL(*v)
	return nil
}
