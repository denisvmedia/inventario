-- Migration generated from schema differences
-- Generated on: 2026-02-26T12:30:50+01:00
-- Direction: UP

-- POSTGRES TABLE: email_verifications --
CREATE TABLE email_verifications (
  id TEXT PRIMARY KEY NOT NULL,
  user_id TEXT NOT NULL,
  tenant_id TEXT NOT NULL,
  email TEXT NOT NULL,
  token TEXT NOT NULL,
  expires_at TIMESTAMP NOT NULL,
  verified_at TIMESTAMP,
  created_at TIMESTAMP NOT NULL
);
CREATE INDEX IF NOT EXISTS email_verifications_email_idx ON email_verifications (email);
CREATE UNIQUE INDEX IF NOT EXISTS email_verifications_token_idx ON email_verifications (token);
CREATE INDEX IF NOT EXISTS email_verifications_user_id_idx ON email_verifications (user_id);
ALTER TABLE email_verifications ADD CONSTRAINT fk_email_verification_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;
ALTER TABLE email_verifications ADD CONSTRAINT fk_email_verification_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE;