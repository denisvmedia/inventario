// Package urlerr turns the error of parsing a URL that carries credentials
// into one that quotes nothing from the URL.
//
// url.Parse quotes the whole URL in its error, and the reason inside that
// error quotes the part the parser could not read. A database DSN or a Redis
// URL carries its password, and a password with a character a URL reserves
// cuts the URL short: in `postgres://app:p#ss@db/x` the "#" starts the
// fragment, and the reason is `invalid port ":p" after host`. Either text
// puts the password, or part of it, into whatever log, panic or CLI message
// reports the error (devdocs/logging.md).
package urlerr

import (
	"errors"
	"net/url"
	"slices"
	"strings"
)

// hint is the fix for the usual cause: a password with a reserved character.
const hint = "percent-encode the characters a URL reserves in the user name and password, such as @ : / ? # and %"

// Error is a URL parse error with fixed text: the kind of problem and how to
// fix it.
type Error struct {
	// Reason is the kind of problem, such as "invalid port after host". It is
	// fixed text and never holds a value taken from the URL.
	Reason string
}

func (e *Error) Error() string {
	return e.Reason + " (" + hint + ")"
}

// Parse is url.Parse with an *Error in place of the error url.Parse returns.
func Parse(rawURL string) (*url.URL, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, WithoutURL(err)
	}
	return u, nil
}

// WithoutURL replaces a *url.Error with an *Error that names the same kind
// of problem, and returns any other error as it is. Apply it to the error
// that comes straight from the parser or the driver: the *Error replaces the
// whole error, including anything that wrapped the *url.Error.
func WithoutURL(err error) error {
	urlErr, ok := errors.AsType[*url.Error](err)
	if !ok {
		return err
	}
	return &Error{Reason: reason(urlErr.Err)}
}

// fixedReasons are the reasons net/url gives as fixed text.
var fixedReasons = []string{
	"empty url",
	"first path segment in URL cannot contain colon",
	"invalid IP-literal",
	"invalid URI for request",
	"missing ']' in host",
	"missing protocol scheme",
	"net/url: invalid control character in URL",
	"net/url: invalid userinfo",
}

// reason names the kind of problem a reason from net/url describes. Only
// fixed text leaves it: a reason that quotes a part of the URL is matched by
// its kind, and one it does not know becomes "malformed URL".
func reason(err error) string {
	if _, ok := errors.AsType[url.EscapeError](err); ok {
		return "invalid URL escape"
	}
	if _, ok := errors.AsType[url.InvalidHostError](err); ok {
		return "invalid character in host name"
	}
	msg := err.Error()
	switch {
	case strings.HasPrefix(msg, "invalid port ") && strings.HasSuffix(msg, " after host"):
		return "invalid port after host"
	case strings.HasPrefix(msg, "invalid host: "):
		return "invalid host"
	case slices.Contains(fixedReasons, msg):
		return msg
	default:
		return "malformed URL"
	}
}
