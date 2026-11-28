// Package redisurl parses a Redis URL without quoting it in the error.
package redisurl

import (
	"errors"
	"net/url"

	"github.com/redis/go-redis/v9"
)

// Parse is redis.ParseURL with an error that leaves the URL out.
//
// url.Parse quotes the whole URL in its error, and a Redis URL carries its
// password, so whoever logged the error of redis.ParseURL logged the password
// too. Parse keeps the cause, such as `invalid port ":x" after host`, and
// drops the URL. The errors go-redis writes itself name the scheme, the path
// or the option that is wrong, never the credentials, and pass through as
// they are.
func Parse(rawURL string) (*redis.Options, error) {
	opts, err := redis.ParseURL(rawURL)
	if err != nil {
		if urlErr, ok := errors.AsType[*url.Error](err); ok {
			return nil, urlErr.Err
		}
		return nil, err
	}
	return opts, nil
}
