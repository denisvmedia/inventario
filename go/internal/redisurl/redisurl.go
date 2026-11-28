// Package redisurl parses a Redis URL with an error that quotes nothing from
// it.
package redisurl

import (
	"errors"
	"net/url"
	"strings"

	"github.com/redis/go-redis/v9"

	"go.5x5.cz/inventario/internal/urlerr"
)

// Parse is redis.ParseURL with an *urlerr.Error in place of its error.
//
// redis.ParseURL returns the error of url.Parse, which quotes the URL, and
// errors of its own that quote the path or an option value. A Redis URL
// carries its password, and a password with a character a URL reserves spills
// into the port, the path or the query, so each of these errors can hold the
// password or part of it. Parse keeps the kind of problem and drops the rest.
func Parse(rawURL string) (*redis.Options, error) {
	opts, err := redis.ParseURL(rawURL)
	if err == nil {
		return opts, nil
	}
	if _, ok := errors.AsType[*url.Error](err); ok {
		return nil, urlerr.WithoutURL(err)
	}
	return nil, &urlerr.Error{Reason: reason(err.Error())}
}

// reasons maps the start of a go-redis error to fixed text, most specific
// first. What follows that start in the error is the value go-redis quotes.
var reasons = []struct{ prefix, reason string }{
	{"redis: invalid URL scheme", "redis: invalid URL scheme, expected redis, rediss or unix"},
	{"redis: invalid URL path", "redis: invalid URL path, expected /<database number>"},
	{"redis: invalid database number", "redis: invalid database number"},
	{"redis: empty unix socket path", "redis: empty unix socket path"},
	{"redis: unexpected option", "redis: unexpected query option"},
	{"redis: invalid ", "redis: invalid query option value"},
}

func reason(msg string) string {
	for _, r := range reasons {
		if strings.HasPrefix(msg, r.prefix) {
			return r.reason
		}
	}
	return "redis: invalid URL"
}
