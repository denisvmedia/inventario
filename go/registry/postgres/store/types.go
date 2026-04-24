package store

import (
	"github.com/denisvmedia/inventario/models"
)

type tenantAware interface {
	GetTenantID() string
	SetTenantID(string)
}

type userAware interface {
	GetUserID() string
	SetUserID(string)
}

type tenantUserAware interface {
	tenantAware
	userAware
	models.IDable
}

type ptrTenantUserAware[T any] interface {
	*T
	tenantUserAware
}

// tenantAwareIDable is the narrower constraint used by RLSRepository itself:
// the repository only requires an ID and a tenant. User-awareness is
// demanded only by NewUserAwareSQLRegistry, which constrains its caller
// through ptrTenantUserAware; in-body user-ID propagation uses a runtime
// type assertion to userAware so that tenant-only entities (location
// groups, group memberships, group invites, invite-audit) can share the
// same RLSRepository plumbing without carrying dummy user_id methods.
type tenantAwareIDable interface {
	tenantAware
	models.IDable
}

type ptrTenantAware[T any] interface {
	*T
	tenantAwareIDable
}

type ptrIDable[T any] interface {
	*T
	models.IDable
}

type groupAware interface {
	GetGroupID() string
	SetGroupID(string)
}

type createdByUserAware interface {
	GetCreatedByUserID() string
	SetCreatedByUserID(string)
}

type tenantGroupAware interface {
	tenantAware
	groupAware
	createdByUserAware
	models.IDable
}

type ptrTenantGroupAware[T any] interface {
	*T
	tenantGroupAware
}
