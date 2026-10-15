package ports_test

import (
	"context"
	"errors"
	"testing"

	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/ports"
)

var (
	_ port.Repository[domain.UserID, domain.UserAccountSnapshot]      = ports.UserAccountRepository(nil)
	_ ports.UserAccountReader                                         = port.Reader[domain.UserID, domain.UserAccountSnapshot](nil)
	_ port.Repository[domain.MembershipID, domain.MembershipSnapshot] = ports.MembershipRepository(nil)
)

type membershipsByUser struct {
	*relation[domain.UserID, domain.MembershipSnapshot]
}

func (r membershipsByUser) LoadByUser(ctx context.Context, userID domain.UserID) ([]domain.MembershipSnapshot, error) {
	return r.load(ctx, userID)
}

type membershipsByOrganization struct {
	*relation[domain.OrganizationID, domain.MembershipSnapshot]
}

func (r membershipsByOrganization) LoadByOrganization(ctx context.Context, organizationID domain.OrganizationID) ([]domain.MembershipSnapshot, error) {
	return r.load(ctx, organizationID)
}

func TestLoadByUserAnswersTheMembershipsOfTheSubjectInEveryOrganization(t *testing.T) {
	held := []domain.MembershipSnapshot{
		{ID: "mem-1", OrganizationID: "org-1", UserID: "user-sub-1"},
		{ID: "mem-2", OrganizationID: "org-2", UserID: "user-sub-1"},
	}
	stub := &relation[domain.UserID, domain.MembershipSnapshot]{held: held}
	var reader ports.MembershipsByUserReader = membershipsByUser{stub}

	got, err := reader.LoadByUser(context.Background(), "user-sub-1")

	if err != nil || len(got) != len(held) {
		t.Fatalf("LoadByUser() = %v, %v; want the memberships of every organization", got, err)
	}
	if len(stub.asked) != 1 || stub.asked[0] != "user-sub-1" {
		t.Fatalf("the reader was keyed on %v, want the subject", stub.asked)
	}
}

func TestLoadByOrganizationAnswersTheMembershipsOfIt(t *testing.T) {
	stub := &relation[domain.OrganizationID, domain.MembershipSnapshot]{held: []domain.MembershipSnapshot{{ID: "mem-1", OrganizationID: "org-1"}}}
	var reader ports.MembershipsByOrganizationReader = membershipsByOrganization{stub}

	got, err := reader.LoadByOrganization(context.Background(), "org-1")

	if err != nil || len(got) != 1 || stub.asked[0] != "org-1" {
		t.Fatalf("LoadByOrganization() = %v, %v after asking %v; want the membership of the organization", got, err, stub.asked)
	}
}

func TestTheMembershipRelationsSurfaceTheCrossTenantAccessUntranslated(t *testing.T) {
	access := port.CrossTenantAccess{Object: "memberships?user_id=user-sub-1", ContextTenant: "globex", DataTenant: "acme"}
	var reader ports.MembershipsByUserReader = membershipsByUser{&relation[domain.UserID, domain.MembershipSnapshot]{err: access}}

	_, err := reader.LoadByUser(context.Background(), "user-sub-1")

	var got port.CrossTenantAccess
	if !errors.As(err, &got) || got != access {
		t.Fatalf("LoadByUser() = %v, want the CrossTenantAccess the caller reads as absent", err)
	}
}
