import { LegalPage, LegalSection } from "./LegalPage"

// Terms for a self-hosted instance: the operator running it sets the actual
// relationship with its users, so this states the software's own terms and
// defers the rest to them. Review before inviting anyone.
export default function TermsPage() {
  return (
    <LegalPage title="Terms of Service" updated="2026-09-20">
      <p>
        Inventario is open-source software, licensed under the MIT License. These terms cover
        your use of this particular instance. The organization running it may add terms of its
        own.
      </p>

      <LegalSection heading="Your account">
        <p>
          You are responsible for what happens under your account, including keeping your
          password and any two-factor codes to yourself. Tell the operator if you believe your
          account has been accessed by someone else.
        </p>
        <p>
          Accounts are for people, not for sharing. If several people need access, invite them
          to your group rather than sharing one login.
        </p>
      </LegalSection>

      <LegalSection heading="Your content">
        <p>
          What you put into Inventario stays yours. Running the software grants the operator no
          ownership of it — only the access needed to store it, serve it back to you, and keep
          backups.
        </p>
        <p>
          Do not upload content you have no right to store, or content that is unlawful where
          this instance is operated.
        </p>
      </LegalSection>

      <LegalSection heading="Availability">
        <p>
          This instance is provided as-is. There is no uptime guarantee unless the operator has
          given you one separately, and the software itself comes with no warranty — see the
          MIT License for the full disclaimer.
        </p>
        <p>
          Keep your own copy of anything you cannot afford to lose. The application can export
          your data at any time.
        </p>
      </LegalSection>

      <LegalSection heading="Ending your use">
        <p>
          You may stop using this instance whenever you like and ask the operator to delete your
          account. The operator may close an account that is being used to break these terms.
        </p>
      </LegalSection>

      <LegalSection heading="Changes">
        <p>
          These terms may change as the software does. The date at the top says when this
          version was written; material changes should be announced by the operator.
        </p>
      </LegalSection>
    </LegalPage>
  )
}
