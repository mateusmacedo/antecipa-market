//go:build integration

package provider_test

import (
	"context"
	"strconv"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/providerkit"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/pg"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/appkit"
	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/provider"
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
	id := domain.PartnerID("kit-event-" + strconv.Itoa(n))
	return port.OutboxEntry{
		MessageID: port.MessageID("evt-" + string(id)), OccurredAt: 1,
		Intent:        port.PublishIntent{Destination: "partner.events", PartitionKey: string(id)},
		AggregateType: "partner.Partner", AggregateID: string(id), AggregateVersion: 1,
		Event: domain.PartnerRegistered{PartnerID: id, OrganizationID: "org-kit", Role: domain.RoleFunder, At: 1},
	}
}

// Every constraint here leads with tenant_id and one attribute, but the one of
// partners, so a suite skips the clauses of a second constraint and of a
// composite key: the two constraints of a versioned table share one rejection,
// which the kit cannot tell apart, and so run as two suites.
const (
	skippedOfComposite = 2
	skippedOfSingle    = 3
)

func requireUniqueKit(t *testing.T, v providerkit.Verdict, skipped int) {
	t.Helper()
	tb.Require(t, v)
	if len(v.Skipped) != skipped {
		t.Fatalf("skipped = %v, want %d: only the clauses of a second constraint or of a composite key", v.Skipped, skipped)
	}
}

// kitPartner holds every row of a guarded table to one partner: the guard
// refuses an update that moves a row to another partner (ADR-067), and a
// series never changes partner.
const kitPartner = domain.PartnerID("prt-kit")

// series puts the declared value of the suite where the constraint reads it:
// supersedes_id holds it as is, and series_origin derives from the attribute
// of a first version that the caller sets from the returned origin.
type series int

const (
	bySupersedes series = iota
	byOrigin
)

func (s series) constraint(table string, code kernel.Code) providerkit.UniqueConstraint {
	column := "supersedes_id"
	if s == byOrigin {
		column = "series_origin"
	}
	return providerkit.UniqueConstraint{Name: table + "_tenant_id_" + column + "_key", Columns: []string{"tenant_id", column}, Code: code}
}

// WHY: an empty supersedes is no supersedes, stored as NULL, which is what
// makes a first version; the empty value of the kit becomes one named version,
// so its clause still proves the collision.
const emptySupersedes = "kit-empty"

func (s series) place(key string, partner *domain.PartnerID, supersedes *string, version *int) (origin string) {
	*partner = kitPartner
	if s == bySupersedes {
		if key == "" {
			key = emptySupersedes
		}
		*supersedes, *version = key, 2
		return ""
	}
	*supersedes, *version = "", 1
	return key
}

const kitBase = 1_755_432_000_000_000_000

func TestPartnerRepositoryHonoursTheUniqueKit(t *testing.T) {
	pool := appkit.OpenPool(t)
	v := providerkit.Unique(func() providerkit.UniqueSubject[domain.PartnerID, domain.PartnerSnapshot] {
		pg.ResetTables(t, pool, appkit.Tables...)
		return providerkit.UniqueSubject[domain.PartnerID, domain.PartnerSnapshot]{
			Within: uniqueWithin(pool, provider.NewPartnerRepository),
			Reader: provider.NewPartnerReader(postgres.NewReadPool(pool)),
			NewID:  func(n int) domain.PartnerID { return domain.PartnerID("kit-" + strconv.Itoa(n)) },
			NewState: func(marker int, keys [][]string) domain.PartnerSnapshot {
				s := partner(marker)
				s.OrganizationID, s.Role = domain.OrganizationID(keys[0][0]), domain.PartnerRole(keys[0][1])
				return s
			},
			Marker: func(s domain.PartnerSnapshot) int { return int(s.StageChangedAt - 1_755_518_400_000_000_000) },
			Table:  partnersTable,
			Constraints: []providerkit.UniqueConstraint{{
				Name: "partners_tenant_id_organization_id_role_key", Columns: []string{"tenant_id", "organization_id", "role"}, Code: domain.CodePartnerAlreadyRegistered,
			}},
			Concurrent: true,
		}
	})
	requireUniqueKit(t, v, skippedOfComposite)
}

// WHY: the series of an agreement is its partner alone, the guard itself, so
// moving a first version onto another series is the update the guard refuses
// as a version conflict; series_origin of agreements is proven by
// TestTheSeriesHoldsOneFirstVersionAndOneSuccessorOfEachVersion and
// TestTwoConcurrentFirstVersionsOfOneSeriesLeaveOneAgreement instead.
func TestAgreementRepositoryHonoursTheUniqueKitOfSupersedes(t *testing.T) {
	s := bySupersedes
	pool := appkit.OpenPool(t)
	v := providerkit.Unique(func() providerkit.UniqueSubject[domain.AgreementID, domain.PartnerAgreementSnapshot] {
		pg.ResetTables(t, pool, appkit.Tables...)
		return providerkit.UniqueSubject[domain.AgreementID, domain.PartnerAgreementSnapshot]{
			Within: uniqueWithin(pool, provider.NewAgreementRepository),
			Reader: provider.NewAgreementReader(postgres.NewReadPool(pool)),
			NewID:  func(n int) domain.AgreementID { return domain.AgreementID("kit-" + strconv.Itoa(n)) },
			NewState: func(marker int, keys [][]string) domain.PartnerAgreementSnapshot {
				a := agreement(marker)
				supersedes := ""
				s.place(keys[0][0], &a.PartnerID, &supersedes, &a.Version)
				a.SupersedesID = domain.AgreementID(supersedes)
				return a
			},
			Marker:      func(a domain.PartnerAgreementSnapshot) int { return int(a.ActivatedAt - 1_755_518_400_000_000_000) },
			Table:       agreementsTable,
			Constraints: []providerkit.UniqueConstraint{s.constraint(agreementsTable, domain.CodePartnerAgreementSupersedesInvalid)},
			Concurrent:  true,
		}
	})
	requireUniqueKit(t, v, skippedOfSingle)
}

func TestIntegrationProfileRepositoryHonoursTheUniqueKitOfItsSeries(t *testing.T) {
	for _, s := range []series{bySupersedes, byOrigin} {
		pool := appkit.OpenPool(t)
		v := providerkit.Unique(func() providerkit.UniqueSubject[domain.ProfileID, domain.IntegrationProfileSnapshot] {
			pg.ResetTables(t, pool, appkit.Tables...)
			return providerkit.UniqueSubject[domain.ProfileID, domain.IntegrationProfileSnapshot]{
				Within: uniqueWithin(pool, provider.NewIntegrationProfileRepository),
				Reader: provider.NewIntegrationProfileReader(postgres.NewReadPool(pool)),
				NewID:  func(n int) domain.ProfileID { return domain.ProfileID("kit-" + strconv.Itoa(n)) },
				NewState: func(marker int, keys [][]string) domain.IntegrationProfileSnapshot {
					p := profile(marker)
					supersedes := ""
					if origin := s.place(keys[0][0], &p.PartnerID, &supersedes, &p.Version); s == byOrigin {
						p.Environment = domain.Environment(origin)
					}
					p.SupersedesID = domain.ProfileID(supersedes)
					return p
				},
				Marker: func(p domain.IntegrationProfileSnapshot) int {
					return int(p.CertificateExpiresAt - 1_786_968_000_000_000_000)
				},
				Table:       profilesTable,
				Constraints: []providerkit.UniqueConstraint{s.constraint(profilesTable, domain.CodeIntegrationProfileSupersedesInvalid)},
				Concurrent:  true,
			}
		})
		requireUniqueKit(t, v, skippedOfSingle)
	}
}

func TestOperationalPolicyRepositoryHonoursTheUniqueKitOfItsSeries(t *testing.T) {
	for _, s := range []series{bySupersedes, byOrigin} {
		pool := appkit.OpenPool(t)
		v := providerkit.Unique(func() providerkit.UniqueSubject[domain.PolicyID, domain.OperationalPolicySnapshot] {
			pg.ResetTables(t, pool, appkit.Tables...)
			return providerkit.UniqueSubject[domain.PolicyID, domain.OperationalPolicySnapshot]{
				Within: uniqueWithin(pool, provider.NewOperationalPolicyRepository),
				Reader: provider.NewOperationalPolicyReader(postgres.NewReadPool(pool)),
				NewID:  func(n int) domain.PolicyID { return domain.PolicyID("kit-" + strconv.Itoa(n)) },
				NewState: func(marker int, keys [][]string) domain.OperationalPolicySnapshot {
					p := policy(marker)
					supersedes := ""
					if origin := s.place(keys[0][0], &p.PartnerID, &supersedes, &p.Version); s == byOrigin {
						p.PartnerID = domain.PartnerID(origin)
					}
					p.SupersedesID = domain.PolicyID(supersedes)
					return p
				},
				Marker:      func(p domain.OperationalPolicySnapshot) int { return int(p.EffectiveFrom - kitBase) },
				Table:       policiesTable,
				Constraints: []providerkit.UniqueConstraint{s.constraint(policiesTable, domain.CodeOperationalPolicySupersedesInvalid)},
				Concurrent:  true,
			}
		})
		requireUniqueKit(t, v, skippedOfSingle)
	}
}

func TestCapabilityRepositoryHonoursTheUniqueKitOfItsSeries(t *testing.T) {
	for _, s := range []series{bySupersedes, byOrigin} {
		pool := appkit.OpenPool(t)
		v := providerkit.Unique(func() providerkit.UniqueSubject[domain.CapabilityID, domain.CapabilitySnapshot] {
			pg.ResetTables(t, pool, appkit.Tables...)
			return providerkit.UniqueSubject[domain.CapabilityID, domain.CapabilitySnapshot]{
				Within: uniqueWithin(pool, provider.NewCapabilityRepository),
				Reader: provider.NewCapabilityReader(postgres.NewReadPool(pool)),
				NewID:  func(n int) domain.CapabilityID { return domain.CapabilityID("kit-" + strconv.Itoa(n)) },
				NewState: func(marker int, keys [][]string) domain.CapabilitySnapshot {
					c := capabilityState(marker)
					supersedes := ""
					if origin := s.place(keys[0][0], &c.PartnerID, &supersedes, &c.Version); s == byOrigin {
						c.Operation = origin
					}
					c.SupersedesID = domain.CapabilityID(supersedes)
					return c
				},
				Marker:      func(c domain.CapabilitySnapshot) int { return int(c.EnabledAt - kitBase) },
				Table:       capabilitiesTable,
				Constraints: []providerkit.UniqueConstraint{s.constraint(capabilitiesTable, domain.CodeCapabilitySupersedesInvalid)},
				Concurrent:  true,
			}
		})
		requireUniqueKit(t, v, skippedOfSingle)
	}
}

func TestFinancialProductRepositoryHonoursTheUniqueKitOfItsSeries(t *testing.T) {
	for _, s := range []series{bySupersedes, byOrigin} {
		pool := appkit.OpenPool(t)
		v := providerkit.Unique(func() providerkit.UniqueSubject[domain.ProductID, domain.FinancialProductSnapshot] {
			pg.ResetTables(t, pool, appkit.Tables...)
			return providerkit.UniqueSubject[domain.ProductID, domain.FinancialProductSnapshot]{
				Within: uniqueWithin(pool, provider.NewFinancialProductRepository),
				Reader: provider.NewFinancialProductReader(postgres.NewReadPool(pool)),
				NewID:  func(n int) domain.ProductID { return domain.ProductID("kit-" + strconv.Itoa(n)) },
				NewState: func(marker int, keys [][]string) domain.FinancialProductSnapshot {
					p := financialProduct(marker)
					supersedes := ""
					if origin := s.place(keys[0][0], &p.PartnerID, &supersedes, &p.Version); s == byOrigin {
						p.Code = origin
					}
					p.SupersedesID = domain.ProductID(supersedes)
					return p
				},
				Marker:      func(p domain.FinancialProductSnapshot) int { return int(p.ValidFrom - kitBase) },
				Table:       productsTable,
				Constraints: []providerkit.UniqueConstraint{s.constraint(productsTable, domain.CodeFinancialProductSupersedesInvalid)},
				Concurrent:  true,
			}
		})
		requireUniqueKit(t, v, skippedOfSingle)
	}
}
