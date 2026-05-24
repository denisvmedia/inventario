package models

import (
	"context"
	"time"

	"github.com/jellydator/validation"

	"github.com/denisvmedia/inventario/models/rules"
)

// OAuthProvider names the identity provider this row was issued by. Values
// are the same strings used in URL paths (`/api/v1/auth/oauth/{provider}/...`)
// and `login_events.method` (`oauth_{provider}`).
type OAuthProvider string

const (
	// OAuthProviderGoogle identifies the Google OAuth 2.0 provider.
	OAuthProviderGoogle OAuthProvider = "google"
	// OAuthProviderGitHub identifies the GitHub OAuth 2.0 provider.
	OAuthProviderGitHub OAuthProvider = "github"
)

// IsValid reports whether p is one of the known providers. Empty / unknown
// values are treated as invalid; handler code uses this to 404 on unknown
// `{provider}` path params before looking anything up.
func (p OAuthProvider) IsValid() bool {
	switch p {
	case OAuthProviderGoogle, OAuthProviderGitHub:
		return true
	}
	return false
}

// OAuthIdentity records a link between an Inventario user and an account at
// an external OAuth provider (#1394). A user can have multiple identities
// linked — one per provider — and a single (provider, provider_user_id) pair
// is globally unique so a provider account can't authenticate to two
// Inventario accounts simultaneously.
//
// The table is tenant-scoped via TenantAwareEntityID so the row lives with
// the rest of the user's data and is hard-deleted on tenant purge. RLS
// allows the user to see only their own rows; the OAuth callback runs in
// the background-worker role because it must look up a row by
// (provider, provider_user_id) BEFORE any user session exists.
//
//migrator:schema:table name="user_oauth_identities"
//migrator:schema:rls:enable table="user_oauth_identities" comment="Enable RLS for multi-tenant OAuth identity isolation"
//migrator:schema:rls:policy name="oauth_identity_user_isolation" table="user_oauth_identities" for="ALL" to="inventario_app" using="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND user_id = get_current_user_id() AND get_current_user_id() IS NOT NULL AND get_current_user_id() != ''" with_check="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND user_id = get_current_user_id() AND get_current_user_id() IS NOT NULL AND get_current_user_id() != ''" comment="Users can read and modify only their own OAuth identities"
//migrator:schema:rls:policy name="oauth_identity_background_worker_access" table="user_oauth_identities" for="ALL" to="inventario_background_worker" using="true" with_check="true" comment="OAuth callback runs before any user session exists; uses background-worker role to look up identities by (provider, provider_user_id)"
type OAuthIdentity struct {
	//migrator:embedded mode="inline"
	TenantAwareEntityID

	// UserID is the Inventario user this identity belongs to. ON DELETE
	// CASCADE: deleting the user removes the identity rows so the
	// (provider, provider_user_id) pair becomes available again for a
	// fresh sign-up.
	//migrator:schema:field name="user_id" type="TEXT" not_null="true" foreign="users(id)" foreign_key_name="fk_oauth_identity_user" on_delete="CASCADE"
	UserID string `json:"user_id" db:"user_id"`

	// Provider is the OAuthProvider enum value ("google" | "github").
	// Stored TEXT so the enum can grow without a CHECK migration each
	// time we add a provider.
	//migrator:schema:field name="provider" type="TEXT" not_null="true"
	Provider OAuthProvider `json:"provider" db:"provider"`

	// ProviderUserID is the stable identifier the provider issues for the
	// account ("sub" claim on Google, numeric "id" on GitHub). NEVER use
	// email or username as the lookup key — both can be reassigned at
	// the provider, while the provider_user_id is documented stable.
	//migrator:schema:field name="provider_user_id" type="TEXT" not_null="true"
	ProviderUserID string `json:"provider_user_id" db:"provider_user_id"`

	// Email is the address the provider returned at link time. Recorded
	// for display in the "Connected accounts" UI; not used as an
	// authentication key. May go stale if the user changes their email at
	// the provider — we don't poll for it.
	//migrator:schema:field name="email" type="TEXT" not_null="true"
	Email string `json:"email" db:"email"`

	// LinkedAt is when the link was created. Read-only in the API.
	//migrator:schema:field name="linked_at" type="TIMESTAMP" not_null="true" default_expr="CURRENT_TIMESTAMP"
	LinkedAt time.Time `json:"linked_at" db:"linked_at" userinput:"false"`
}

// OAuthIdentityIndexes carries the migrator hints for the table's indexes.
type OAuthIdentityIndexes struct {
	// Unique index for the immutable UUID (deduplication key for
	// import/restore — same shape every other tenant-scoped table uses).
	//migrator:schema:index name="idx_oauth_identities_uuid" fields="uuid" unique="true" table="user_oauth_identities"
	_ int

	// Global uniqueness on (provider, provider_user_id) — a single
	// provider account cannot link to two Inventario accounts. The unique
	// constraint must NOT include tenant_id: same Google account
	// authenticating across tenants is a feature we explicitly do not
	// support, since the "log in via Google" handler has no tenant
	// context to scope the lookup.
	//migrator:schema:index name="idx_oauth_identities_provider_subject" fields="provider,provider_user_id" unique="true" table="user_oauth_identities"
	_ int

	// Per-user lookup ("show me everything linked to this user").
	//migrator:schema:index name="idx_oauth_identities_user_id" fields="user_id" table="user_oauth_identities"
	_ int

	// Tenant isolation index — same shape every other tenant-scoped
	// table uses; lets the RLS qual short-circuit on a single column.
	//migrator:schema:index name="idx_oauth_identities_tenant_id" fields="tenant_id" table="user_oauth_identities"
	_ int
}

var (
	_ validation.Validatable            = (*OAuthIdentity)(nil)
	_ validation.ValidatableWithContext = (*OAuthIdentity)(nil)
	_ TenantAwareIDable                 = (*OAuthIdentity)(nil)
)

func (*OAuthIdentity) Validate() error {
	return ErrMustUseValidateWithContext
}

func (i *OAuthIdentity) ValidateWithContext(ctx context.Context) error {
	return validation.ValidateStructWithContext(ctx, i,
		validation.Field(&i.UserID, rules.NotEmpty),
		validation.Field(&i.TenantID, rules.NotEmpty),
		validation.Field(&i.ProviderUserID, rules.NotEmpty),
		validation.Field(&i.Email, rules.NotEmpty),
		validation.Field(&i.Provider, validation.By(func(value any) error {
			p, ok := value.(OAuthProvider)
			if !ok || !p.IsValid() {
				return validation.NewError("validation_oauth_provider_invalid", "unsupported OAuth provider")
			}
			return nil
		})),
	)
}
