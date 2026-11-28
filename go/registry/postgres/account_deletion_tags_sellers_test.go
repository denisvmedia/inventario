package postgres_test

import (
	"context"
	"encoding/json"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/services"
)

// A tag or seller a member created in a shared group belongs to the group
// (#2883): the deletion hands it to the owner instead of refusing with
// ErrAccountStillOwnsContent, and writes one audit row for the group in the
// purge transaction, with the request id and the operator of an
// impersonation session.
func TestAccountDeletion_Postgres_MemberWhoCreatedTagsAndSellers(t *testing.T) {
	c := qt.New(t)
	f := newLoanPGFixture(t)
	ctx := context.Background()
	pool, err := getOrCreatePool(skipIfNoPostgreSQL(t))
	c.Assert(err, qt.IsNil)
	db := sqlx.NewDb(stdlib.OpenDBFromPool(pool), "pgx")
	fs := f.factorySet
	owner := f.user

	_, err = fs.GroupMembershipRegistry.Create(ctx, models.GroupMembership{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: owner.TenantID},
		GroupID:             f.groupAID,
		MemberUserID:        owner.ID,
		Role:                models.GroupRoleOwner,
	})
	c.Assert(err, qt.IsNil)
	member, err := fs.UserRegistry.Create(ctx, models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: owner.TenantID},
		Email:               "tagging-member@test-org.com",
		Name:                "Tagging Member",
		IsActive:            true,
	})
	c.Assert(err, qt.IsNil)
	_, err = fs.GroupMembershipRegistry.Create(ctx, models.GroupMembership{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: owner.TenantID},
		GroupID:             f.groupAID,
		MemberUserID:        member.ID,
		Role:                models.GroupRoleUser,
	})
	c.Assert(err, qt.IsNil)

	memberCtx := loanCtxFor(member, f.groupAID)
	tags, err := fs.TagRegistryFactory.CreateUserRegistry(memberCtx)
	c.Assert(err, qt.IsNil)
	tag, err := tags.Create(memberCtx, models.Tag{Kind: models.TagKindCommodity, Slug: "garage-shelf", Label: "Garage shelf", Color: models.TagColorBlue})
	c.Assert(err, qt.IsNil)
	sellers, err := fs.SellerRegistryFactory.CreateUserRegistry(memberCtx)
	c.Assert(err, qt.IsNil)
	seller, err := sellers.Create(memberCtx, models.Seller{Name: "Hardware store"})
	c.Assert(err, qt.IsNil)

	deleteCtx := appctx.WithRequestID(ctx, "req-2883")
	deleteCtx = appctx.WithJWTClaims(deleteCtx, jwt.MapClaims{"imp": true, "impersonator_id": "operator-1"})
	fileSvc := services.NewFileService(fs, newPostgresFileUploadLocation(c))
	deletion := services.NewAccountDeletionService(fs, services.NewGroupPurgeService(fs, fileSvc), nil)
	c.Assert(deletion.DeleteAccount(deleteCtx, owner.TenantID, member.ID), qt.IsNil)

	_, err = fs.UserRegistry.Get(ctx, member.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
	c.Check(creatorOf(c, db, "tags", "id = $1", tag.ID), qt.Equals, owner.ID)
	c.Check(creatorOf(c, db, "sellers", "id = $1", seller.ID), qt.Equals, owner.ID)

	var row struct {
		UserID         string  `db:"user_id"`
		TenantID       string  `db:"tenant_id"`
		EntityType     string  `db:"entity_type"`
		UserAgent      string  `db:"user_agent"`
		RequestID      *string `db:"request_id"`
		ImpersonatedBy *string `db:"impersonated_by"`
	}
	c.Assert(db.GetContext(ctx, &row,
		`SELECT user_id, tenant_id, entity_type, user_agent, request_id, impersonated_by
		FROM audit_logs WHERE action = $1 AND entity_id = $2`,
		registry.AuditActionAccountDeletionHandOver, f.groupAID), qt.IsNil)
	c.Check(row.UserID, qt.Equals, member.ID)
	c.Check(row.TenantID, qt.Equals, owner.TenantID)
	c.Check(row.EntityType, qt.Equals, "location_group")
	c.Check(row.RequestID, qt.DeepEquals, new("req-2883"))
	c.Check(row.ImpersonatedBy, qt.DeepEquals, new("operator-1"))
	var handOver struct {
		ToUserID string         `json:"to_user_id"`
		Rows     map[string]int `json:"rows"`
	}
	c.Assert(json.Unmarshal([]byte(row.UserAgent), &handOver), qt.IsNil)
	c.Check(handOver.ToUserID, qt.Equals, owner.ID)
	c.Check(handOver.Rows, qt.DeepEquals, map[string]int{"tags": 1, "sellers": 1})
}

// A custom field a member created in a shared group goes to the owner with
// its item type entries, and the audit row counts both tables, as the memory
// purger does.
func TestAccountDeletion_Postgres_MemberWhoCreatedCustomFields(t *testing.T) {
	c := qt.New(t)
	f := newLoanPGFixture(t)
	ctx := context.Background()
	pool, err := getOrCreatePool(skipIfNoPostgreSQL(t))
	c.Assert(err, qt.IsNil)
	db := sqlx.NewDb(stdlib.OpenDBFromPool(pool), "pgx")
	fs := f.factorySet
	owner := f.user

	_, err = fs.GroupMembershipRegistry.Create(ctx, models.GroupMembership{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: owner.TenantID},
		GroupID:             f.groupAID,
		MemberUserID:        owner.ID,
		Role:                models.GroupRoleOwner,
	})
	c.Assert(err, qt.IsNil)
	member, err := fs.UserRegistry.Create(ctx, models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: owner.TenantID},
		Email:               "field-member@test-org.com",
		Name:                "Field Member",
		IsActive:            true,
	})
	c.Assert(err, qt.IsNil)
	_, err = fs.GroupMembershipRegistry.Create(ctx, models.GroupMembership{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: owner.TenantID},
		GroupID:             f.groupAID,
		MemberUserID:        member.ID,
		Role:                models.GroupRoleUser,
	})
	c.Assert(err, qt.IsNil)

	memberCtx := loanCtxFor(member, f.groupAID)
	types, err := fs.ItemTypeRegistryFactory.CreateUserRegistry(memberCtx)
	c.Assert(err, qt.IsNil)
	_, err = types.EnsureBuiltins(memberCtx)
	c.Assert(err, qt.IsNil)
	other, err := types.GetBySlug(memberCtx, models.CommodityTypeOther)
	c.Assert(err, qt.IsNil)
	fields, err := fs.CustomFieldRegistryFactory.CreateUserRegistry(memberCtx)
	c.Assert(err, qt.IsNil)
	field, err := fields.Create(memberCtx, models.CustomFieldDefinition{
		Name: "Serial", Type: models.CustomFieldTypeText,
		Templates: []models.CustomFieldTemplate{{ItemTypeID: other.ID}},
	})
	c.Assert(err, qt.IsNil)

	fileSvc := services.NewFileService(fs, newPostgresFileUploadLocation(c))
	deletion := services.NewAccountDeletionService(fs, services.NewGroupPurgeService(fs, fileSvc), nil)
	c.Assert(deletion.DeleteAccount(ctx, owner.TenantID, member.ID), qt.IsNil)

	c.Check(creatorOf(c, db, "custom_field_definitions", "id = $1", field.ID), qt.Equals, owner.ID)
	c.Check(creatorOf(c, db, "item_type_custom_fields", "field_id = $1", field.ID), qt.Equals, owner.ID)

	var breadcrumb string
	c.Assert(db.GetContext(ctx, &breadcrumb,
		`SELECT user_agent FROM audit_logs WHERE action = $1 AND entity_id = $2`,
		registry.AuditActionAccountDeletionHandOver, f.groupAID), qt.IsNil)
	var handOver struct {
		Rows map[string]int `json:"rows"`
	}
	c.Assert(json.Unmarshal([]byte(breadcrumb), &handOver), qt.IsNil)
	c.Check(handOver.Rows, qt.DeepEquals, map[string]int{"custom_field_definitions": 1, "item_type_custom_fields": 1})
}
