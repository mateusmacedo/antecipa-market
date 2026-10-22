//go:build integration

package provider_test

import (
	"context"
	"strconv"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/providerkit"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/pg"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/appkit"
	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/provider"
)

// uniqueWithin opens the transaction the kit writes in, with the repository of
// the table under test and an outbox for the clause that needs a prior write.
func uniqueWithin[ID comparable, S any](pool *pgxpool.Pool, repository func(*postgres.Tx) port.Repository[ID, S]) func(context.Context, func(context.Context, providerkit.UniqueWrites[ID, S]) error) error {
	return func(ctx context.Context, fn func(context.Context, providerkit.UniqueWrites[ID, S]) error) error {
		uow := postgres.NewUnitOfWork(pool, func(tx *postgres.Tx) *postgres.Tx { return tx })
		return uow.Within(ctx, func(ctx context.Context, tx *postgres.Tx) error {
			return fn(ctx, providerkit.UniqueWrites[ID, S]{
				Repo: repository(tx),
				Enqueue: func(ctx context.Context, n int) error {
					return tx.Outbox(provider.Mapper{}).Enqueue(ctx, kitEntry(n))
				},
			})
		})
	}
}

func kitEntry(n int) port.OutboxEntry {
	id := domain.OrganizationID("kit-event-" + strconv.Itoa(n))
	return port.OutboxEntry{
		MessageID: port.MessageID("evt-" + string(id)), OccurredAt: 1,
		Intent:        port.PublishIntent{Destination: "identity.events", PartitionKey: string(id)},
		AggregateType: "identity.Organization", AggregateID: string(id), AggregateVersion: 1,
		Event: domain.OrganizationRegistered{OrganizationID: id, Kinds: domain.KindsOf(domain.KindClient), Jurisdiction: "BR", TaxIDType: domain.TaxIDTypeCNPJ, At: 1},
	}
}

// requireUniqueKit allows only the clauses that need a constraint this table
// does not have: a second one, or one over a single attribute.
func requireUniqueKit(t *testing.T, v providerkit.Verdict, skipped int) {
	t.Helper()
	tb.Require(t, v)
	if len(v.Skipped) != skipped {
		t.Fatalf("skipped = %v, want %d: only the clauses of a second constraint or of a composite key", v.Skipped, skipped)
	}
}

func TestOrganizationRepositoryHonoursTheUniqueKit(t *testing.T) {
	pool := appkit.OpenPool(t)
	v := providerkit.Unique(func() providerkit.UniqueSubject[domain.OrganizationID, domain.OrganizationSnapshot] {
		pg.ResetTables(t, pool, appkit.Tables...)
		return providerkit.UniqueSubject[domain.OrganizationID, domain.OrganizationSnapshot]{
			Within: uniqueWithin(pool, provider.NewOrganizationRepository),
			Reader: provider.NewOrganizationReader(postgres.NewReadPool(pool)),
			NewID:  func(n int) domain.OrganizationID { return domain.OrganizationID("kit-" + strconv.Itoa(n)) },
			NewState: func(marker int, keys [][]string) domain.OrganizationSnapshot {
				s := organization(marker)
				s.TaxIDHash, s.Jurisdiction = keys[0][0], keys[0][1]
				return s
			},
			Marker: func(s domain.OrganizationSnapshot) int { return s.TaxIDHashKeyVersion },
			Table:  organizationsTable,
			Constraints: []providerkit.UniqueConstraint{{
				Name: "organizations_tenant_id_tax_id_hash_jurisdiction_key", Columns: []string{"tenant_id", "tax_id_hash", "jurisdiction"}, Code: domain.CodeOrganizationDuplicateTaxID,
			}},
			Concurrent: true,
		}
	})
	requireUniqueKit(t, v, 2)
}

func TestEstablishmentRepositoryHonoursTheUniqueKit(t *testing.T) {
	pool := appkit.OpenPool(t)
	v := providerkit.Unique(func() providerkit.UniqueSubject[domain.EstablishmentID, domain.EstablishmentSnapshot] {
		pg.ResetTables(t, pool, appkit.Tables...)
		return providerkit.UniqueSubject[domain.EstablishmentID, domain.EstablishmentSnapshot]{
			Within: uniqueWithin(pool, provider.NewEstablishmentRepository),
			Reader: establishmentReader(pool),
			NewID:  func(n int) domain.EstablishmentID { return domain.EstablishmentID("kit-" + strconv.Itoa(n)) },
			NewState: func(marker int, keys [][]string) domain.EstablishmentSnapshot {
				s := establishment(marker)
				s.OrganizationID, s.TaxIDHash = domain.OrganizationID(keys[0][0]), keys[0][1]
				return s
			},
			Marker: func(s domain.EstablishmentSnapshot) int { return s.TaxIDHashKeyVersion },
			Table:  establishmentsTable,
			Constraints: []providerkit.UniqueConstraint{{
				Name: "establishments_tenant_id_organization_id_tax_id_hash_key", Columns: []string{"tenant_id", "organization_id", "tax_id_hash"}, Code: domain.CodeEstablishmentDuplicateTaxID,
			}},
			Concurrent: true,
		}
	})
	requireUniqueKit(t, v, 2)
}

func TestPersonRepositoryHonoursTheUniqueKit(t *testing.T) {
	pool := appkit.OpenPool(t)
	v := providerkit.Unique(func() providerkit.UniqueSubject[domain.PersonID, domain.PersonSnapshot] {
		pg.ResetTables(t, pool, appkit.Tables...)
		return providerkit.UniqueSubject[domain.PersonID, domain.PersonSnapshot]{
			Within: uniqueWithin(pool, provider.NewPersonRepository),
			Reader: provider.NewPersonReader(postgres.NewReadPool(pool)),
			NewID:  func(n int) domain.PersonID { return domain.PersonID("kit-" + strconv.Itoa(n)) },
			NewState: func(marker int, keys [][]string) domain.PersonSnapshot {
				s := person(marker)
				s.DocumentHash = keys[0][0]
				return s
			},
			Marker: func(s domain.PersonSnapshot) int { return s.DocumentHashKeyVersion },
			Table:  personsTable,
			Constraints: []providerkit.UniqueConstraint{{
				Name: "persons_tenant_id_document_hash_key", Columns: []string{"tenant_id", "document_hash"}, Code: domain.CodePersonDuplicateDocument,
			}},
			Concurrent: true,
		}
	})
	requireUniqueKit(t, v, 3)
}
