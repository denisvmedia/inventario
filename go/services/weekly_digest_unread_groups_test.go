package services_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

var errRegistryDown = errors.New("registry down")

// membersUnreadable fails ListByGroup for one group, as a database error in
// the middle of a sweep would.
type membersUnreadable struct {
	registry.GroupMembershipRegistry
	groupID string
}

func (r membersUnreadable) ListByGroup(ctx context.Context, groupID string) ([]*models.GroupMembership, error) {
	if groupID == r.groupID {
		return nil, errRegistryDown
	}
	return r.GroupMembershipRegistry.ListByGroup(ctx, groupID)
}

// tenantUnreadableOnce fails the first tenant lookup and answers the rest.
type tenantUnreadableOnce struct {
	registry.TenantRegistry
	mu     sync.Mutex
	failed bool
}

func (r *tenantUnreadableOnce) Get(ctx context.Context, id string) (*models.Tenant, error) {
	r.mu.Lock()
	first := !r.failed
	r.failed = true
	r.mu.Unlock()
	if first {
		return nil, errRegistryDown
	}
	return r.TenantRegistry.Get(ctx, id)
}

// addMember creates another active user of the fixture's tenant and makes
// them a member of group.
func (f *digestFixture) addMember(c *qt.C, group *models.LocationGroup, email string) *models.User {
	c.Helper()
	ctx := context.Background()

	user, err := f.factorySet.UserRegistry.Create(ctx, models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: f.tenantID},
		Email:               email,
		Name:                "Other Member",
		IsActive:            true,
	})
	c.Assert(err, qt.IsNil)

	_, err = f.factorySet.GroupMembershipRegistry.Create(ctx, models.GroupMembership{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: f.tenantID},
		GroupID:             group.ID,
		MemberUserID:        user.ID,
		Role:                models.GroupRoleUser,
	})
	c.Assert(err, qt.IsNil)
	return user
}

func digestGroupNames(d recordedDigest) []string {
	names := make([]string, 0, len(d.digest.Groups))
	for _, g := range d.digest.Groups {
		names = append(names, g.GroupName)
	}
	return names
}

// A user in a group whose members could not be listed gets no digest in that
// sweep, and the week is not claimed: sending the other groups alone would
// lose the unread group's week for good. A user whose groups were all read
// gets their digest as usual.
func TestWeeklyDigestService_HoldsBackTheDigestWhenAGroupsMembersCannotBeRead(t *testing.T) {
	c := qt.New(t)
	f := newDigestFixture(c)
	household := f.addGroup(c, "Household")
	cabin := f.addGroup(c, "Cabin")
	f.addEvent(c, household, f.addCommodity(c, household, "Drill", nil).ID, models.CommodityEventKindCreated, f.window.Start.Add(time.Hour))
	f.addEvent(c, cabin, f.addCommodity(c, cabin, "Kayak", nil).ID, models.CommodityEventKindCreated, f.window.Start.Add(time.Hour))
	other := f.addMember(c, household, "other@example.com")

	members := f.factorySet.GroupMembershipRegistry
	f.factorySet.GroupMembershipRegistry = membersUnreadable{GroupMembershipRegistry: members, groupID: cabin.ID}
	email := &recordingDigestEmailService{}
	svc := f.newService(email)

	stats, err := svc.SendOnce(context.Background(), f.sentAt)
	c.Assert(err, qt.IsNil)
	c.Assert(stats.Sent, qt.Equals, 1)
	c.Assert(stats.Failed, qt.Equals, 2, qt.Commentf("the unread group and the held-back user"))
	sent := email.recorded()
	c.Assert(sent, qt.HasLen, 1)
	c.Assert(sent[0].to, qt.Equals, other.Email)

	// The members can be read again: the held-back user gets the full digest.
	f.factorySet.GroupMembershipRegistry = members
	stats, err = svc.SendOnce(context.Background(), f.sentAt)
	c.Assert(err, qt.IsNil)
	c.Assert(stats.Sent, qt.Equals, 1)
	c.Assert(stats.AlreadySent, qt.Equals, 1)
	sent = email.recorded()
	c.Assert(sent, qt.HasLen, 2)
	c.Assert(sent[1].to, qt.Equals, f.email)
	c.Assert(digestGroupNames(sent[1]), qt.ContentEquals, []string{"Household", "Cabin"})
}

// The same holds when a group's tenant could not be read.
func TestWeeklyDigestService_HoldsBackTheDigestWhenAGroupsTenantCannotBeRead(t *testing.T) {
	c := qt.New(t)
	f := newDigestFixture(c)
	household := f.addGroup(c, "Household")
	cabin := f.addGroup(c, "Cabin")
	f.addEvent(c, household, f.addCommodity(c, household, "Drill", nil).ID, models.CommodityEventKindCreated, f.window.Start.Add(time.Hour))
	f.addEvent(c, cabin, f.addCommodity(c, cabin, "Kayak", nil).ID, models.CommodityEventKindCreated, f.window.Start.Add(time.Hour))

	f.factorySet.TenantRegistry = &tenantUnreadableOnce{TenantRegistry: f.factorySet.TenantRegistry}
	email := &recordingDigestEmailService{}
	svc := f.newService(email)

	stats, err := svc.SendOnce(context.Background(), f.sentAt)
	c.Assert(err, qt.IsNil)
	c.Assert(stats.Sent, qt.Equals, 0)
	c.Assert(stats.Failed, qt.Equals, 2, qt.Commentf("the unread group and the held-back user"))
	c.Assert(email.recorded(), qt.HasLen, 0)

	stats, err = svc.SendOnce(context.Background(), f.sentAt)
	c.Assert(err, qt.IsNil)
	c.Assert(stats.Sent, qt.Equals, 1)
	sent := email.recorded()
	c.Assert(sent, qt.HasLen, 1)
	c.Assert(digestGroupNames(sent[0]), qt.ContentEquals, []string{"Household", "Cabin"})
}
