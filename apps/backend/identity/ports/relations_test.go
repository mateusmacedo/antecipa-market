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

type establishmentsByTaxID struct {
	*relation[string, domain.EstablishmentSnapshot]
}

func (r establishmentsByTaxID) LoadByTaxIDHash(ctx context.Context, taxIDHash string) ([]domain.EstablishmentSnapshot, error) {
	return r.load(ctx, taxIDHash)
}

type establishmentsByOrganization struct {
	*relation[domain.OrganizationID, domain.EstablishmentSnapshot]
}

func (r establishmentsByOrganization) LoadByOrganization(ctx context.Context, organizationID domain.OrganizationID) ([]domain.EstablishmentSnapshot, error) {
	return r.load(ctx, organizationID)
}

type personsByDocument struct {
	*relation[string, domain.PersonSnapshot]
}

func (r personsByDocument) LoadByDocumentHash(ctx context.Context, documentHash string) ([]domain.PersonSnapshot, error) {
	return r.load(ctx, documentHash)
}

type representationsByOrganization struct {
	*relation[domain.OrganizationID, domain.RepresentationSnapshot]
}

func (r representationsByOrganization) LoadByOrganization(ctx context.Context, organizationID domain.OrganizationID) ([]domain.RepresentationSnapshot, error) {
	return r.load(ctx, organizationID)
}

func TestLoadByTaxIDHashOfEstablishmentsAnswersEveryOrganizationOfTheHash(t *testing.T) {
	held := []domain.EstablishmentSnapshot{
		{ID: "est-1", OrganizationID: "org-1", TaxIDHash: "h-1"},
		{ID: "est-2", OrganizationID: "org-2", TaxIDHash: "h-1"},
	}
	stub := &relation[string, domain.EstablishmentSnapshot]{held: held}
	var reader ports.EstablishmentsByTaxIDReader = establishmentsByTaxID{stub}

	got, err := reader.LoadByTaxIDHash(context.Background(), "h-1")

	if err != nil || len(got) != len(held) {
		t.Fatalf("LoadByTaxIDHash() = %v, %v; want every organization of the hash: the caller refines it", got, err)
	}
	if len(stub.asked) != 1 || stub.asked[0] != "h-1" {
		t.Fatalf("the reader was keyed on %v, want the hash: the tax id in clear never reaches it", stub.asked)
	}
}

func TestLoadByDocumentHashIsKeyedOnTheHash(t *testing.T) {
	stub := &relation[string, domain.PersonSnapshot]{held: []domain.PersonSnapshot{{ID: "per-1", DocumentHash: "h-1"}}}
	var reader ports.PersonsByDocumentReader = personsByDocument{stub}

	got, err := reader.LoadByDocumentHash(context.Background(), "h-1")

	if err != nil || len(got) != 1 || stub.asked[0] != "h-1" {
		t.Fatalf("LoadByDocumentHash() = %v, %v after asking %v; want the person of the hash", got, err, stub.asked)
	}
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
	access := port.CrossTenantAccess{Object: "persons?document_hash=h-1", ContextTenant: "globex", DataTenant: "acme"}
	var reader ports.PersonsByDocumentReader = personsByDocument{&relation[string, domain.PersonSnapshot]{err: access}}

	_, err := reader.LoadByDocumentHash(context.Background(), "h-1")

	var got port.CrossTenantAccess
	if !errors.As(err, &got) || got != access {
		t.Fatalf("LoadByDocumentHash() = %v, want the CrossTenantAccess the caller reads as absent", err)
	}
}
