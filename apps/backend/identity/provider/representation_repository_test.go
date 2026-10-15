//go:build integration

package provider_test

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/providerkit"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/pg"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/appkit"
	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/provider"
)

const representationsTable = "representations"

func representation(marker int) domain.RepresentationSnapshot {
	return domain.RepresentationSnapshot{
		OrganizationID:      repoOrganizationID,
		PersonID:            "per-1",
		PowerType:           domain.PowerAttorneyInFact,
		SourceType:          domain.SourcePowerOfAttorney,
		SourceEvidenceID:    "evd-1",
		ScopeCurrency:       "BRL",
		ScopeMaxAmountMinor: int64(marker),
		ScopeActTypes:       []string{"assignment", "signature"},
		ValidFrom:           kernel.Instant(1_755_000_000_000_000_000),
		ValidUntil:          kernel.Instant(1_786_536_000_000_000_000),
		VerificationStatus:  domain.RepresentationDeclared,
		ProposedByUserID:    "user-sub-1",
	}
}

func saveRepresentation(ctx context.Context, pool *pgxpool.Pool, id domain.RepresentationID, s domain.RepresentationSnapshot) error {
	return pg.Within(ctx, pool, provider.NewRepresentationRepository, func(ctx context.Context, repo port.Repository[domain.RepresentationID, domain.RepresentationSnapshot]) error {
		return repo.Save(ctx, id, s, 0)
	})
}

func TestRepresentationRepositoryConformsToTheKit(t *testing.T) {
	pool := appkit.OpenPool(t)
	v := providerkit.Repository(func() providerkit.RepositorySubject[domain.RepresentationID, domain.RepresentationSnapshot] {
		pg.ResetTables(t, pool, representationsTable)
		return providerkit.RepositorySubject[domain.RepresentationID, domain.RepresentationSnapshot]{
			Within: func(ctx context.Context, fn func(context.Context, port.Repository[domain.RepresentationID, domain.RepresentationSnapshot]) error) error {
				return pg.Within(ctx, pool, provider.NewRepresentationRepository, fn)
			},
			Reader:           provider.NewRepresentationReader(postgres.NewReadPool(pool)),
			NewID:            func(n int) domain.RepresentationID { return domain.RepresentationID("kit-" + strconv.Itoa(n)) },
			NewState:         representation,
			Marker:           func(s domain.RepresentationSnapshot) int { return int(s.ScopeMaxAmountMinor) },
			TenantUnresolved: postgres.ErrTenantUnresolved,
			Concurrent:       true,
		}
	})
	tb.Require(t, v)
	if len(v.Skipped) != 0 {
		t.Fatalf("postgres scopes by construction and runs transactions at once; nothing should be skipped: %v", v.Skipped)
	}
}

func TestRepresentationRoundTripsTheWholeState(t *testing.T) {
	pool := appkit.OpenPool(t)
	ctx := withExecution(t, context.Background())
	reader := provider.NewRepresentationReader(postgres.NewReadPool(pool))
	bounded := representation(1)
	indefinite := representation(0)
	indefinite.ScopeCurrency, indefinite.ScopeActTypes, indefinite.ValidUntil = "", nil, 0

	for id, s := range map[domain.RepresentationID]domain.RepresentationSnapshot{"rep-bounded": bounded, "rep-indefinite": indefinite} {
		if err := saveRepresentation(ctx, pool, id, s); err != nil {
			t.Fatalf("Save(%s) = %v", id, err)
		}
		got, version, err := reader.Load(ctx, id)
		s.ID = id
		if err != nil || version != 1 || !got.Equal(s) {
			t.Fatalf("Load(%s) = %+v v%d, %v; want %+v v1", id, got, version, err, s)
		}
		if got.ScopeActTypes == nil {
			t.Fatalf("Load(%s).ScopeActTypes = nil, want an empty collection", id)
		}
	}
}

func representationIDs(snapshots []domain.RepresentationSnapshot) []string {
	out := make([]string, 0, len(snapshots))
	for _, s := range snapshots {
		out = append(out, string(s.ID))
	}
	slices.Sort(out)
	return out
}

func TestLoadByOrganizationReturnsTheRepresentationsOfTheOrganization(t *testing.T) {
	pool := appkit.OpenPool(t)
	ctx := withExecution(t, context.Background())
	elsewhere := representation(3)
	elsewhere.OrganizationID = "org-other"
	for id, s := range map[domain.RepresentationID]domain.RepresentationSnapshot{
		"rep-a": representation(1), "rep-b": representation(2), "rep-c": elsewhere,
	} {
		if err := saveRepresentation(ctx, pool, id, s); err != nil {
			t.Fatalf("save %s: %v", id, err)
		}
	}
	reader := provider.NewRepresentationsByOrganizationReader(postgres.NewReadPool(pool))

	found, err := reader.LoadByOrganization(ctx, repoOrganizationID)

	if got, want := representationIDs(found), []string{"rep-a", "rep-b"}; err != nil || !slices.Equal(got, want) {
		t.Fatalf("LoadByOrganization() = %v, %v; want %v", got, err, want)
	}
	for _, s := range found {
		want := representation(int(s.ScopeMaxAmountMinor))
		want.ID = s.ID
		if !s.Equal(want) {
			t.Fatalf("representation %+v lost state, want %+v", s, want)
		}
	}
	_, err = reader.LoadByOrganization(withTenant(t, context.Background(), "globex"), repoOrganizationID)
	var access port.CrossTenantAccess
	if !errors.As(err, &access) || access.DataTenant != "acme" {
		t.Fatalf("LoadByOrganization() from another tenant = %v, want a CrossTenantAccess", err)
	}
}
