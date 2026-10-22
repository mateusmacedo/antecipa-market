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
	_ port.Repository[domain.EstablishmentID, domain.EstablishmentSnapshot]   = ports.EstablishmentRepository(nil)
	_ port.Repository[domain.PersonID, domain.PersonSnapshot]                 = ports.PersonRepository(nil)
	_ ports.PersonReader                                                      = port.Reader[domain.PersonID, domain.PersonSnapshot](nil)
	_ port.Repository[domain.RepresentationID, domain.RepresentationSnapshot] = ports.RepresentationRepository(nil)
	_ ports.RepresentationReader                                              = port.Reader[domain.RepresentationID, domain.RepresentationSnapshot](nil)
)

type relation[K ~string, S any] struct {
	asked []K
	held  []S
	err   error
}

func (r *relation[K, S]) load(_ context.Context, key K) ([]S, error) {
	r.asked = append(r.asked, key)
	return r.held, r.err
}

type establishmentsByOrganization struct {
	*relation[domain.OrganizationID, domain.EstablishmentSnapshot]
}

func (r establishmentsByOrganization) LoadByOrganization(ctx context.Context, organizationID domain.OrganizationID) ([]domain.EstablishmentSnapshot, error) {
	return r.load(ctx, organizationID)
}

type representationsByOrganization struct {
	*relation[domain.OrganizationID, domain.RepresentationSnapshot]
}

func (r representationsByOrganization) LoadByOrganization(ctx context.Context, organizationID domain.OrganizationID) ([]domain.RepresentationSnapshot, error) {
	return r.load(ctx, organizationID)
}

func TestLoadByOrganizationAnswersTheEstablishmentsAndTheRepresentationsOfIt(t *testing.T) {
	establishments := &relation[domain.OrganizationID, domain.EstablishmentSnapshot]{held: []domain.EstablishmentSnapshot{{ID: "est-1", OrganizationID: "org-1"}}}
	representations := &relation[domain.OrganizationID, domain.RepresentationSnapshot]{held: []domain.RepresentationSnapshot{{ID: "rep-1", OrganizationID: "org-1"}}}
	var (
		byOrganization        ports.EstablishmentsByOrganizationReader  = establishmentsByOrganization{establishments}
		representationsReader ports.RepresentationsByOrganizationReader = representationsByOrganization{representations}
	)

	gotEstablishments, errEstablishments := byOrganization.LoadByOrganization(context.Background(), "org-1")
	gotRepresentations, errRepresentations := representationsReader.LoadByOrganization(context.Background(), "org-1")

	if errEstablishments != nil || len(gotEstablishments) != 1 || errRepresentations != nil || len(gotRepresentations) != 1 {
		t.Fatalf("LoadByOrganization() = %v, %v and %v, %v; want one of each", gotEstablishments, errEstablishments, gotRepresentations, errRepresentations)
	}
}

func TestTheRelationsSurfaceTheCrossTenantAccessUntranslated(t *testing.T) {
	access := port.CrossTenantAccess{Object: "establishments?organization_id=org-1", ContextTenant: "globex", DataTenant: "acme"}
	var reader ports.EstablishmentsByOrganizationReader = establishmentsByOrganization{&relation[domain.OrganizationID, domain.EstablishmentSnapshot]{err: access}}

	_, err := reader.LoadByOrganization(context.Background(), "org-1")

	var got port.CrossTenantAccess
	if !errors.As(err, &got) || got != access {
		t.Fatalf("LoadByOrganization() = %v, want the CrossTenantAccess the caller reads as absent", err)
	}
}
