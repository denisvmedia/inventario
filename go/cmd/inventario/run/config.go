package run

import (
	"github.com/denisvmedia/inventario/internal/defaults"
)

type Config struct {
	Addr                          string `yaml:"addr" env:"ADDR" env-default:":3333"`
	UploadLocation                string `yaml:"upload_location" env:"UPLOAD_LOCATION" env-default:""`
	MaxConcurrentExports          int    `yaml:"max_concurrent_exports" env:"MAX_CONCURRENT_EXPORTS" env-default:"0"`
	MaxConcurrentImports          int    `yaml:"max_concurrent_imports" env:"MAX_CONCURRENT_IMPORTS" env-default:"0"`
	JWTSecret                     string `yaml:"jwt_secret" env:"JWT_SECRET" env-default:""`
	FileSigningKey                string `yaml:"file_signing_key" env:"FILE_SIGNING_KEY" env-default:""`
	FileURLExpiration             string `yaml:"file_url_expiration" env:"FILE_URL_EXPIRATION" env-default:"15m"`
	ThumbnailMaxConcurrentPerUser int    `yaml:"thumbnail_max_concurrent_per_user" env:"THUMBNAIL_MAX_CONCURRENT_PER_USER" env-default:"0"`
	ThumbnailRateLimitPerMinute   int    `yaml:"thumbnail_rate_limit_per_minute" env:"THUMBNAIL_RATE_LIMIT_PER_MINUTE" env-default:"0"`
	ThumbnailSlotDuration         string `yaml:"thumbnail_slot_duration" env:"THUMBNAIL_SLOT_DURATION" env-default:"30m"`
	TokenBlacklistRedisURL        string `yaml:"token_blacklist_redis_url" env:"TOKEN_BLACKLIST_REDIS_URL" env-default:""`
	AuthRateLimitRedisURL         string `yaml:"auth_rate_limit_redis_url" env:"AUTH_RATE_LIMIT_REDIS_URL" env-default:""`
	AuthRateLimitDisabled         bool   `yaml:"auth_rate_limit_disabled" env:"AUTH_RATE_LIMIT_DISABLED" env-default:"false"`
}

func (c *Config) setDefaults() {
	if c.Addr == "" {
		c.Addr = defaults.GetServerAddr()
	}
	if c.UploadLocation == "" {
		c.UploadLocation = defaults.GetUploadLocation()
	}
	if c.MaxConcurrentExports == 0 {
		c.MaxConcurrentExports = defaults.GetMaxConcurrentExports()
	}
	if c.MaxConcurrentImports == 0 {
		c.MaxConcurrentImports = defaults.GetMaxConcurrentImports()
	}
	if c.JWTSecret == "" {
		c.JWTSecret = defaults.GetJWTSecret()
	}
	if c.ThumbnailMaxConcurrentPerUser == 0 {
		c.ThumbnailMaxConcurrentPerUser = defaults.GetThumbnailMaxConcurrentPerUser()
	}
	if c.ThumbnailRateLimitPerMinute == 0 {
		c.ThumbnailRateLimitPerMinute = defaults.GetThumbnailRateLimitPerMinute()
	}
	if c.ThumbnailSlotDuration == "" {
		c.ThumbnailSlotDuration = defaults.GetThumbnailSlotDuration()
	}
}
