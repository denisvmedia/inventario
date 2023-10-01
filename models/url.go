package models

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

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

type URLs []*URL

func (u *URLs) Validate() error {
	if u == nil {
		return nil
	}

	if err := validation.Validate(*u); err != nil {
		return fmt.Errorf("invalid urls: %w", err)
	}

	return nil
}

func (u *URLs) MarshalJSON() ([]byte, error) {
	if u == nil {
		return []byte("null"), nil
	}

	tmp := make([]string, 0, len(*u))
	for _, v := range *u {
		tmp = append(tmp, v.String())
	}

	return json.Marshal(strings.Join(tmp, "\n"))
}

func (u *URLs) UnmarshalJSON(data []byte) error {
	var tmp string
	err := json.Unmarshal(data, &tmp)
	if err != nil {
		return err
	}

	surls := strings.Split(tmp, "\n")
	*u = make([]*URL, 0, len(surls))

	for _, el := range surls {
		s := strings.TrimSpace(el)
		if s == "" {
			continue
		}

		parsed, err := URLParse(s)
		if err != nil {
			return err
		}

		*u = append(*u, parsed)
	}

	return nil
}
