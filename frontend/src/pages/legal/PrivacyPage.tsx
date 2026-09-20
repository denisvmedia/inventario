import { LegalPage, LegalSection } from "./LegalPage"

// The text below describes what this software actually stores and sends,
// verified against the models and the outbound integrations. An operator who
// self-hosts is the data controller and has to review it against their own
// deployment and jurisdiction before inviting users — the defaults it
// describes can be changed by configuration.
export default function PrivacyPage() {
  return (
    <LegalPage title="Privacy Policy" updated="2026-09-20">
      <p>
        Inventario is self-hosted software. The organization running this instance decides how it is
        configured and is responsible for the data in it. This page describes what the software
        stores and what it can send elsewhere.
      </p>

      <LegalSection heading="What is stored about you">
        <p>
          Your account holds an email address, a name, a password hash (bcrypt — the password itself
          is never stored), whether the account is active, and the time you last signed in.
        </p>
        <p>
          Sign-in attempts are recorded with the email used, the outcome, the method, your IP
          address, your browser&apos;s user-agent string and a timestamp. This is what lets you
          review your own login history and lets an administrator investigate a compromised account.
        </p>
        <p>
          Everything you enter about your inventory — locations, areas, items, prices, tags, and any
          files you upload such as photos, manuals and invoices — is stored as you entered it.
        </p>
      </LegalSection>

      <LegalSection heading="Who can see it">
        <p>
          Your data is scoped to your organization and the groups you belong to. The database
          enforces this with row-level security, not only the application.
        </p>
        <p>
          An administrator of this instance can access data across organizations, including by
          impersonating an account for support. Such actions are recorded in an audit log.
        </p>
      </LegalSection>

      <LegalSection heading="What leaves this instance">
        <p>
          By default, nothing except email. Transactional messages — verification, password reset,
          invitations — are delivered through whatever mail provider this instance is configured to
          use.
        </p>
        <p>
          Two integrations are off unless the operator turns them on: error reporting, which sends
          diagnostic data about failures, and AI-assisted item scanning, which sends a photo you
          choose to scan to an external model provider. Ask the operator of this instance which are
          enabled.
        </p>
      </LegalSection>

      <LegalSection heading="How long it is kept">
        <p>
          Your data is kept until you or an administrator deletes it. Deleting an item deletes its
          files. Sign-in history is retained on a schedule the operator configures.
        </p>
      </LegalSection>

      <LegalSection heading="Your choices">
        <p>
          You can view and correct your account details and your inventory at any time from within
          the application, and you can export your data.
        </p>
        <p>
          For deletion of your account, or any request this page does not cover, contact the
          operator of this instance.
        </p>
      </LegalSection>
    </LegalPage>
  )
}
