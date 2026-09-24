package bootstrap

// White-box: resolveGoogleOverrides and resolveGitHubOverrides are unexported,
// and the rule they enforce is the reason they exist — see crypto_internal_test.go
// for the same choice in this package.

import (
	"testing"

	qt "github.com/frankban/quicktest"
)

// A partial override set is refused rather than partially applied. The failure
// it prevents is not a broken test run: the stub's authorize URL combined with
// the real provider token endpoint sends the authorization code and the client
// secret to the provider, while the stub is what the flow expects to answer.
// So each of these cases has to be an error, not a best-effort merge.
func TestResolveGoogleOverrides(t *testing.T) {
	const (
		auth     = "http://127.0.0.1:1/authorize"
		token    = "http://127.0.0.1:1/token"
		userinfo = "http://127.0.0.1:1/userinfo"
	)

	t.Run("none set is the production shape", func(t *testing.T) {
		c := qt.New(t)
		got, err := resolveGoogleOverrides(&Config{})
		c.Assert(err, qt.IsNil)
		c.Check(got, qt.Equals, googleOverrides{})
	})

	t.Run("all three set resolve together", func(t *testing.T) {
		c := qt.New(t)
		got, err := resolveGoogleOverrides(&Config{
			OAuthGoogleAuthURLOverride:     auth,
			OAuthGoogleTokenURLOverride:    token,
			OAuthGoogleUserInfoURLOverride: userinfo,
		})
		c.Assert(err, qt.IsNil)
		c.Check(got, qt.Equals, googleOverrides{AuthURL: auth, TokenURL: token, UserInfoURL: userinfo})
	})

	for _, tc := range []struct {
		name string
		cfg  Config
	}{
		{"auth alone", Config{OAuthGoogleAuthURLOverride: auth}},
		{"token alone", Config{OAuthGoogleTokenURLOverride: token}},
		{"userinfo alone", Config{OAuthGoogleUserInfoURLOverride: userinfo}},
		{"auth and token without userinfo", Config{
			OAuthGoogleAuthURLOverride:  auth,
			OAuthGoogleTokenURLOverride: token,
		}},
	} {
		t.Run(tc.name+" is refused", func(t *testing.T) {
			c := qt.New(t)
			_, err := resolveGoogleOverrides(&tc.cfg)
			c.Assert(err, qt.IsNotNil)
			c.Check(err.Error(), qt.Contains, "must set auth, token, and userinfo together")
		})
	}

	// Whitespace is not a value. A variable exported as an empty string with a
	// stray newline would otherwise count towards the four and turn a
	// production config into a refused one.
	t.Run("whitespace does not count as set", func(t *testing.T) {
		c := qt.New(t)
		got, err := resolveGoogleOverrides(&Config{OAuthGoogleAuthURLOverride: "  \n"})
		c.Assert(err, qt.IsNil)
		c.Check(got, qt.Equals, googleOverrides{})
	})
}

// GitHub takes four endpoints rather than three: the profile and the verified
// email addresses come from separate URLs (#1929).
func TestResolveGitHubOverrides(t *testing.T) {
	const (
		auth   = "http://127.0.0.1:1/gh/authorize"
		token  = "http://127.0.0.1:1/gh/token"
		user   = "http://127.0.0.1:1/gh/user"
		emails = "http://127.0.0.1:1/gh/user/emails"
	)
	all := Config{
		OAuthGitHubAuthURLOverride:       auth,
		OAuthGitHubTokenURLOverride:      token,
		OAuthGitHubUserURLOverride:       user,
		OAuthGitHubUserEmailsURLOverride: emails,
	}

	t.Run("none set is the production shape", func(t *testing.T) {
		c := qt.New(t)
		got, err := resolveGitHubOverrides(&Config{})
		c.Assert(err, qt.IsNil)
		c.Check(got, qt.Equals, githubOverrides{})
	})

	t.Run("all four set resolve together", func(t *testing.T) {
		c := qt.New(t)
		got, err := resolveGitHubOverrides(&all)
		c.Assert(err, qt.IsNil)
		c.Check(got, qt.Equals, githubOverrides{
			AuthURL: auth, TokenURL: token, UserURL: user, UserEmailsURL: emails,
		})
	})

	// Every one-missing case: dropping the emails URL alone is the one most
	// likely to be written by hand, since three of four looks complete next to
	// Google's triple.
	for _, tc := range []struct {
		name string
		drop func(*Config)
	}{
		{"without auth", func(c *Config) { c.OAuthGitHubAuthURLOverride = "" }},
		{"without token", func(c *Config) { c.OAuthGitHubTokenURLOverride = "" }},
		{"without user", func(c *Config) { c.OAuthGitHubUserURLOverride = "" }},
		{"without user-emails", func(c *Config) { c.OAuthGitHubUserEmailsURLOverride = "" }},
	} {
		t.Run(tc.name+" is refused", func(t *testing.T) {
			c := qt.New(t)
			cfg := all
			tc.drop(&cfg)
			_, err := resolveGitHubOverrides(&cfg)
			c.Assert(err, qt.IsNotNil)
			c.Check(err.Error(), qt.Contains, "must set auth, token, user, and user-emails together")
		})
	}

	t.Run("whitespace does not count as set", func(t *testing.T) {
		c := qt.New(t)
		got, err := resolveGitHubOverrides(&Config{OAuthGitHubUserURLOverride: "\t"})
		c.Assert(err, qt.IsNil)
		c.Check(got, qt.Equals, githubOverrides{})
	})
}
