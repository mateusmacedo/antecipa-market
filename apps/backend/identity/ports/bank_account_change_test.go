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
	_ port.Repository[domain.BankAccountChangeID, domain.BankAccountChangeSnapshot] = ports.BankAccountChangeRepository(nil)
	_ ports.BankAccountChangeReader                                                 = port.Reader[domain.BankAccountChangeID, domain.BankAccountChangeSnapshot](nil)
)

type bankAccountChangesByOrganization struct {
	*relation[domain.OrganizationID, domain.BankAccountChangeSnapshot]
}

func (r bankAccountChangesByOrganization) LoadByOrganization(ctx context.Context, organizationID domain.OrganizationID) ([]domain.BankAccountChangeSnapshot, error) {
	return r.load(ctx, organizationID)
}

func TestLoadByOrganizationAnswersTheBankAccountChangesOfIt(t *testing.T) {
	held := []domain.BankAccountChangeSnapshot{
		{ID: "bac-1", OrganizationID: "org-1", Status: domain.BankAccountChangeStatusRequested},
		{ID: "bac-2", OrganizationID: "org-1", Status: domain.BankAccountChangeStatusApproved},
	}
	stub := &relation[domain.OrganizationID, domain.BankAccountChangeSnapshot]{held: held}
	var reader ports.BankAccountChangesByOrganizationReader = bankAccountChangesByOrganization{stub}

	got, err := reader.LoadByOrganization(context.Background(), "org-1")

	if err != nil || len(got) != len(held) {
		t.Fatalf("LoadByOrganization() = %v, %v; want every change of the organization, in any status", got, err)
	}
	if len(stub.asked) != 1 || stub.asked[0] != "org-1" {
		t.Fatalf("the reader was keyed on %v, want the organization", stub.asked)
	}
}

func TestTheBankAccountChangeRelationSurfacesTheCrossTenantAccessUntranslated(t *testing.T) {
	access := port.CrossTenantAccess{Object: "bank_account_changes?organization_id=org-1", ContextTenant: "globex", DataTenant: "acme"}
	var reader ports.BankAccountChangesByOrganizationReader = bankAccountChangesByOrganization{
		&relation[domain.OrganizationID, domain.BankAccountChangeSnapshot]{err: access},
	}

	_, err := reader.LoadByOrganization(context.Background(), "org-1")

	var got port.CrossTenantAccess
	if !errors.As(err, &got) || got != access {
		t.Fatalf("LoadByOrganization() = %v, want the CrossTenantAccess the caller reads as absent", err)
	}
}
