//go:build integration

package provider_test

import (
	"context"
	"encoding/json"
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

const membershipsTable = "memberships"

func invitation(marker int) domain.MembershipSnapshot {
	return domain.MembershipSnapshot{
		OrganizationID:      repoOrganizationID,
		InviteeEmail:        "convidada@sintetica.test",
		Role:                domain.RoleOperator,
		Status:              domain.MembershipStatusInvited,
		InvitedByUserID:     "user-sub-1",
		InvitedAt:           kernel.Instant(1_755_432_000_000_000_000),
		InvitationExpiresAt: kernel.Instant(1_755_432_000_000_000_000 + int64(marker)),
	}
}

func grantedMembership(marker int, user domain.UserID) domain.MembershipSnapshot {
	s := invitation(marker)
	s.Status = domain.MembershipStatusActive
	s.UserID = user
	s.PersonID = "per-1"
	s.VerificationLevel = domain.VerificationPersonalDataChecked
	s.GrantedAt = kernel.Instant(1_755_518_400_000_000_000)
	return s
}

func membershipReader(pool *pgxpool.Pool) repositoryReader[domain.MembershipID, domain.MembershipSnapshot] {
	return repositoryReader[domain.MembershipID, domain.MembershipSnapshot]{pool: pool, repository: provider.NewMembershipRepository}
}

func saveMembership(ctx context.Context, pool *pgxpool.Pool, id domain.MembershipID, s domain.MembershipSnapshot) error {
	return pg.Within(ctx, pool, provider.NewMembershipRepository, func(ctx context.Context, repo port.Repository[domain.MembershipID, domain.MembershipSnapshot]) error {
		return repo.Save(ctx, id, s, 0)
	})
}

func TestMembershipRepositoryConformsToTheKit(t *testing.T) {
	pool := appkit.OpenPool(t)
	v := providerkit.Repository(func() providerkit.RepositorySubject[domain.MembershipID, domain.MembershipSnapshot] {
		pg.ResetTables(t, pool, membershipsTable)
		return providerkit.RepositorySubject[domain.MembershipID, domain.MembershipSnapshot]{
			Within: func(ctx context.Context, fn func(context.Context, port.Repository[domain.MembershipID, domain.MembershipSnapshot]) error) error {
				return pg.Within(ctx, pool, provider.NewMembershipRepository, fn)
			},
			Reader:   membershipReader(pool),
			NewID:    func(n int) domain.MembershipID { return domain.MembershipID("kit-" + strconv.Itoa(n)) },
			NewState: func(marker int) domain.MembershipSnapshot { return grantedMembership(marker, "user-sub-1") },
			Marker: func(s domain.MembershipSnapshot) int {
				return int(int64(s.InvitationExpiresAt) - 1_755_432_000_000_000_000)
			},
			TenantUnresolved: postgres.ErrTenantUnresolved,
			Concurrent:       true,
		}
	})
	tb.Require(t, v)
	if len(v.Skipped) != 0 {
		t.Fatalf("postgres scopes by construction and runs transactions at once; nothing should be skipped: %v", v.Skipped)
	}
}

func TestMembershipRoundTripsTheWholeState(t *testing.T) {
	pool := appkit.OpenPool(t)
	ctx := withExecution(t, context.Background())
	reader := membershipReader(pool)
	revoked := grantedMembership(3, "user-sub-1")
	revoked.Status = domain.MembershipStatusRevoked
	revoked.RevokedAt = kernel.Instant(1_755_604_800_000_000_000)
	revoked.RevocationReason = "saiu da empresa"

	for id, s := range map[domain.MembershipID]domain.MembershipSnapshot{
		"mem-invited": invitation(1), "mem-active": grantedMembership(2, "user-sub-1"), "mem-revoked": revoked,
	} {
		if err := saveMembership(ctx, pool, id, s); err != nil {
			t.Fatalf("Save(%s) = %v", id, err)
		}
		got, version, err := reader.Load(ctx, id)
		s.ID = id
		if err != nil || version != 1 || !got.Equal(s) {
			t.Fatalf("Load(%s) = %+v v%d, %v; want %+v v1", id, got, version, err, s)
		}
	}
}

func TestAnInvitationHasNoUserInTheTypedColumn(t *testing.T) {
	pool := appkit.OpenPool(t)
	ctx := withExecution(t, context.Background())
	if err := saveMembership(ctx, pool, "mem-invited", invitation(1)); err != nil {
		t.Fatalf("Save() = %v", err)
	}

	var userID *string
	if err := pool.QueryRow(ctx, "SELECT user_id FROM memberships WHERE membership_id = $1", "mem-invited").Scan(&userID); err != nil || userID != nil {
		t.Fatalf("user_id = %v, %v; want NULL until the grant", userID, err)
	}
}

func TestTheMembershipSnapshotKeepsItsStoredLayout(t *testing.T) {
	pool := appkit.OpenPool(t)
	ctx := withExecution(t, context.Background())
	if err := saveMembership(ctx, pool, "mem-active", grantedMembership(1, "user-sub-1")); err != nil {
		t.Fatalf("Save() = %v", err)
	}

	var raw []byte
	if err := pool.QueryRow(ctx, "SELECT snapshot FROM memberships WHERE membership_id = $1", "mem-active").Scan(&raw); err != nil {
		t.Fatalf("select snapshot: %v", err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatalf("decode snapshot: %v", err)
	}
	got := make([]string, 0, len(fields))
	for key := range fields {
		got = append(got, key)
	}
	slices.Sort(got)
	want := []string{
		"grantedAt", "invitationExpiresAt", "invitedAt", "invitedByUserId", "inviteeEmail", "personId",
		"revocationReason", "revokedAt", "role", "status", "verificationLevel",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("snapshot keys = %v, want %v", got, want)
	}
}

func TestLoadByUserReturnsTheMembershipsOfTheSubjectInTheTenant(t *testing.T) {
	pool := appkit.OpenPool(t)
	ctx := withExecution(t, context.Background())
	elsewhere := grantedMembership(3, "user-sub-1")
	elsewhere.OrganizationID = "org-other"
	for id, s := range map[domain.MembershipID]domain.MembershipSnapshot{
		"mem-a": grantedMembership(1, "user-sub-1"), "mem-b": elsewhere,
		"mem-c": grantedMembership(2, "user-sub-2"), "mem-d": invitation(4),
	} {
		if err := saveMembership(ctx, pool, id, s); err != nil {
			t.Fatalf("save %s: %v", id, err)
		}
	}
	reader := provider.NewMembershipsByUserReader(postgres.NewReadPool(pool))

	found, err := reader.LoadByUser(ctx, "user-sub-1")

	if got, want := membershipIDs(found), []string{"mem-a", "mem-b"}; err != nil || !slices.Equal(got, want) {
		t.Fatalf("LoadByUser() = %v, %v; want %v", got, err, want)
	}
	if none, err := reader.LoadByUser(ctx, ""); err != nil || len(none) != 0 {
		t.Fatalf("LoadByUser(\"\") = %v, %v; want nothing: an invitation has no user", membershipIDs(none), err)
	}
	_, err = reader.LoadByUser(withTenant(t, context.Background(), "globex"), "user-sub-1")
	var access port.CrossTenantAccess
	if !errors.As(err, &access) || access.DataTenant != "acme" {
		t.Fatalf("LoadByUser() from another tenant = %v, want a CrossTenantAccess", err)
	}
}

func TestLoadByOrganizationReturnsTheMembershipsOfTheOrganization(t *testing.T) {
	pool := appkit.OpenPool(t)
	ctx := withExecution(t, context.Background())
	elsewhere := invitation(3)
	elsewhere.OrganizationID = "org-other"
	for id, s := range map[domain.MembershipID]domain.MembershipSnapshot{
		"mem-a": grantedMembership(1, "user-sub-1"), "mem-b": invitation(2), "mem-c": elsewhere,
	} {
		if err := saveMembership(ctx, pool, id, s); err != nil {
			t.Fatalf("save %s: %v", id, err)
		}
	}
	reader := provider.NewMembershipsByOrganizationReader(postgres.NewReadPool(pool))

	found, err := reader.LoadByOrganization(ctx, repoOrganizationID)

	if got, want := membershipIDs(found), []string{"mem-a", "mem-b"}; err != nil || !slices.Equal(got, want) {
		t.Fatalf("LoadByOrganization() = %v, %v; want %v", got, err, want)
	}
	held := map[domain.MembershipID]domain.MembershipSnapshot{"mem-a": grantedMembership(1, "user-sub-1"), "mem-b": invitation(2)}
	for _, s := range found {
		want := held[s.ID]
		want.ID = s.ID
		if !s.Equal(want) {
			t.Fatalf("membership %+v lost state, want %+v", s, want)
		}
	}
	_, err = reader.LoadByOrganization(withTenant(t, context.Background(), "globex"), repoOrganizationID)
	var access port.CrossTenantAccess
	if !errors.As(err, &access) || access.DataTenant != "acme" {
		t.Fatalf("LoadByOrganization() from another tenant = %v, want a CrossTenantAccess", err)
	}
}

func membershipIDs(snapshots []domain.MembershipSnapshot) []string {
	out := make([]string, 0, len(snapshots))
	for _, s := range snapshots {
		out = append(out, string(s.ID))
	}
	slices.Sort(out)
	return out
}
