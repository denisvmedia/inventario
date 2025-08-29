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

type ptrIDable[T any] interface {
	*T
	models.IDable
}
