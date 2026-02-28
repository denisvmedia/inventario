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
	CSRFRedisURL                  string `yaml:"csrf_redis_url" env:"CSRF_REDIS_URL" env-default:""`
	AllowedOrigins                string `yaml:"allowed_origins" env:"ALLOWED_ORIGINS" env-default:""`
	RegistrationMode              string `yaml:"registration_mode" env:"REGISTRATION_MODE" env-default:"open"`
	PublicURL                     string `yaml:"public_url" env:"PUBLIC_URL" env-default:""`

	EmailProvider        string `yaml:"email_provider" env:"EMAIL_PROVIDER" env-default:"stub"`
	EmailFrom            string `yaml:"email_from" env:"EMAIL_FROM" env-default:""`
	EmailReplyTo         string `yaml:"email_reply_to" env:"EMAIL_REPLY_TO" env-default:""`
	EmailQueueRedisURL   string `yaml:"email_queue_redis_url" env:"EMAIL_QUEUE_REDIS_URL" env-default:""`
	EmailQueueWorkers    int    `yaml:"email_queue_workers" env:"EMAIL_QUEUE_WORKERS" env-default:"5"`
	EmailQueueMaxRetries int    `yaml:"email_queue_max_retries" env:"EMAIL_QUEUE_MAX_RETRIES" env-default:"5"`
	SMTPHost             string `yaml:"smtp_host" env:"SMTP_HOST" env-default:""`
	SMTPPort             int    `yaml:"smtp_port" env:"SMTP_PORT" env-default:"587"`
	SMTPUsername         string `yaml:"smtp_username" env:"SMTP_USERNAME" env-default:""`
	SMTPPassword         string `yaml:"smtp_password" env:"SMTP_PASSWORD" env-default:""`
	SMTPUseTLS           bool   `yaml:"smtp_use_tls" env:"SMTP_USE_TLS" env-default:"true"`
	SendGridAPIKey       string `yaml:"sendgrid_api_key" env:"SENDGRID_API_KEY" env-default:""`
	SendGridBaseURL      string `yaml:"sendgrid_base_url" env:"SENDGRID_BASE_URL" env-default:"https://api.sendgrid.com"`
	AWSRegion            string `yaml:"aws_region" env:"AWS_REGION" env-default:""`
	MandrillAPIKey       string `yaml:"mandrill_api_key" env:"MANDRILL_API_KEY" env-default:""`
	MandrillBaseURL      string `yaml:"mandrill_base_url" env:"MANDRILL_BASE_URL" env-default:"https://mandrillapp.com"`
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
	if c.EmailQueueWorkers <= 0 {
		c.EmailQueueWorkers = 5
	}
	if c.EmailQueueMaxRetries <= 0 {
		c.EmailQueueMaxRetries = 5
	}
	if c.SMTPPort == 0 {
		c.SMTPPort = 587
	}
}
