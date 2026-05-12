package models

import "regexp"

// EmailPattern is the shared regex used to validate user-facing email
// addresses (User.Email, GroupInvite.InviteeEmail, etc.). Kept loose on
// purpose — it catches obvious typos like missing @ or missing TLD while
// letting through anything an SMTP server could plausibly accept. Use
// validation.Match(EmailPattern) inside a ValidateWithContext block.
var EmailPattern = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
