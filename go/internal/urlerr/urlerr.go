// Package urlerr keeps a URL out of the error url.Parse returns for it.
//
// url.Parse quotes the whole URL in its error. A database DSN carries its
// password in that URL, so the error carries it too, into whatever log, panic
// or CLI message reports it (devdocs/logging.md).
package urlerr

import (
	"errors"
	"net/url"
)

// Parse is url.Parse with an error that leaves the URL out.
func Parse(rawURL string) (*url.URL, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, WithoutURL(err)
	}
	return u, nil
}

// WithoutURL returns the cause of a *url.Error, such as
// `invalid URL escape "%zz"`, and any other error as it is. Apply it to the
// error that comes straight from the parser or the driver: the cause replaces
// the whole error, including anything that wrapped it.
func WithoutURL(err error) error {
	if urlErr, ok := errors.AsType[*url.Error](err); ok {
		return urlErr.Err
	}
	return err
}
