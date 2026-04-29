import { ComingSoonBanner } from "@/components/coming-soon"

// Visible 2FA placeholder linked to #1380. Thin wrapper around the shared
// ComingSoonBanner so the auth pages can keep using `<TwoFactorStub />`
// while the registry (#1417) owns the tracker number, icon, and copy.
//
// The data-testid is fixed at "two-factor-stub" for the existing
// auth-page tests; ComingSoonBanner exposes a `testId` override slot so
// callers don't have to rely on the default `coming-soon-banner-twoFactor`.
export function TwoFactorStub() {
  return <ComingSoonBanner surface="twoFactor" testId="two-factor-stub" />
}
