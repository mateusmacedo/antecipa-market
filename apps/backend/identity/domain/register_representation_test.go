package domain_test

import (
	"slices"
	"testing"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
)

const (
	representationID = domain.RepresentationID("rep-100")
	evidenceID       = "evd-100"
	proposerID       = "user-sub-1"
	validFrom        = kernel.Instant(1_755_000_000_000_000_000)
	validUntil       = kernel.Instant(1_786_536_000_000_000_000)
)

func registerRepresentationCommand() domain.RegisterRepresentation {
	return domain.RegisterRepresentation{
		OrganizationID:      organizationID,
		OrganizationStatus:  domain.OrganizationActive,
		PersonID:            personID,
		PersonFound:         true,
		PowerType:           domain.PowerAttorneyInFact,
		SourceType:          domain.SourcePowerOfAttorney,
		SourceEvidenceID:    evidenceID,
		ScopeCurrency:       "BRL",
		ScopeMaxAmountMinor: 150_000,
		ScopeActTypes:       []string{"assignment", "signature"},
		ValidFrom:           validFrom,
		ValidUntil:          validUntil,
		ProposedByUserID:    proposerID,
	}
}

func declaredRepresentation() domain.RepresentationSnapshot {
	return domain.RepresentationSnapshot{
		ID:                  representationID,
		OrganizationID:      organizationID,
		PersonID:            personID,
		PowerType:           domain.PowerAttorneyInFact,
		SourceType:          domain.SourcePowerOfAttorney,
		SourceEvidenceID:    evidenceID,
		ScopeCurrency:       "BRL",
		ScopeMaxAmountMinor: 150_000,
		ScopeActTypes:       []string{"assignment", "signature"},
		ValidFrom:           validFrom,
		ValidUntil:          validUntil,
		VerificationStatus:  domain.RepresentationDeclared,
		ProposedByUserID:    proposerID,
	}
}

func TestRegisterRepresentationDeclaresItWithTheCommandFields(t *testing.T) {
	r := domain.NewRepresentation(representationID)

	acc, rej := r.Register(registerRepresentationCommand())

	tb.RequireAccepted(t, rej)
	if got, want := acc.Response(), (domain.RegisterRepresentationResponse{RepresentationID: representationID}); got != want {
		t.Fatalf("Response() = %+v, want %+v", got, want)
	}
	if events := acc.Events(); len(events) != 0 {
		t.Fatalf("Events() = %v, want none: the verified fact is born in VerifyRepresentation", events)
	}
	if got := r.Snapshot(); !got.Equal(declaredRepresentation()) {
		t.Fatalf("Snapshot() = %+v, want %+v", got, declaredRepresentation())
	}
}

func TestRegisterRepresentationAcceptsEachShapeOfScopeAndValidity(t *testing.T) {
	for name, mutate := range map[string]func(*domain.RegisterRepresentation){
		"no scope at all": func(c *domain.RegisterRepresentation) {
			c.ScopeCurrency, c.ScopeMaxAmountMinor, c.ScopeActTypes = "", 0, nil
		},
		"a scope of act types alone": func(c *domain.RegisterRepresentation) {
			c.ScopeCurrency, c.ScopeMaxAmountMinor = "", 0
		},
		"an amount without act types": func(c *domain.RegisterRepresentation) { c.ScopeActTypes = nil },
		"an indefinite validity":      func(c *domain.RegisterRepresentation) { c.ValidUntil = 0 },
		"each power and source": func(c *domain.RegisterRepresentation) {
			c.PowerType, c.SourceType = domain.PowerLegalRepresentative, domain.SourceArticlesOfAssociation
		},
	} {
		t.Run(name, func(t *testing.T) {
			cmd := registerRepresentationCommand()
			mutate(&cmd)

			_, rej := domain.NewRepresentation(representationID).Register(cmd)

			tb.RequireAccepted(t, rej)
		})
	}
}

func TestRegisterRepresentationStoresTheCurrencyItValidated(t *testing.T) {
	for name, tt := range map[string]struct {
		currency string
		amount   int64
		want     string
	}{
		"blanks without an amount are no ceiling": {"   ", 0, ""},
		"a padded code is the code":               {" BRL ", 150_000, "BRL"},
	} {
		t.Run(name, func(t *testing.T) {
			cmd := registerRepresentationCommand()
			cmd.ScopeCurrency, cmd.ScopeMaxAmountMinor = tt.currency, tt.amount
			r := domain.NewRepresentation(representationID)

			_, rej := r.Register(cmd)

			tb.RequireAccepted(t, rej)
			if got := r.Snapshot().ScopeCurrency; got != tt.want {
				t.Fatalf("ScopeCurrency = %q, want %q: the value validated is the value stored", got, tt.want)
			}
		})
	}
}

func TestRegisterRepresentationRejectsEachPrecondition(t *testing.T) {
	organization := []kernel.Detail{{Key: "organization", Value: string(organizationID)}}
	person := []kernel.Detail{{Key: "person", Value: string(personID)}}
	tests := []struct {
		name    string
		mutate  func(*domain.RegisterRepresentation)
		code    kernel.Code
		details []kernel.Detail
	}{
		{"source evidence empty", func(c *domain.RegisterRepresentation) { c.SourceEvidenceID = "" }, domain.CodeRepresentationSourceEvidenceMissing, nil},
		{"source evidence of blanks", func(c *domain.RegisterRepresentation) { c.SourceEvidenceID = "  " }, domain.CodeRepresentationSourceEvidenceMissing, nil},
		{"valid until equal to valid from", func(c *domain.RegisterRepresentation) { c.ValidUntil = c.ValidFrom }, domain.CodeRepresentationValidityInvalid, nil},
		{"valid until before valid from", func(c *domain.RegisterRepresentation) { c.ValidUntil = c.ValidFrom - 1 }, domain.CodeRepresentationValidityInvalid, nil},
		{"amount without currency", func(c *domain.RegisterRepresentation) { c.ScopeCurrency = "" }, domain.CodeRepresentationScopeInvalid, nil},
		{"amount with a currency of blanks", func(c *domain.RegisterRepresentation) { c.ScopeCurrency = "  " }, domain.CodeRepresentationScopeInvalid, nil},
		{"currency without amount", func(c *domain.RegisterRepresentation) { c.ScopeMaxAmountMinor = 0 }, domain.CodeRepresentationScopeInvalid, nil},
		{"negative amount", func(c *domain.RegisterRepresentation) { c.ScopeMaxAmountMinor = -1 }, domain.CodeRepresentationScopeInvalid, nil},
		{"organization not found", func(c *domain.RegisterRepresentation) { c.OrganizationStatus = domain.OrganizationNew }, domain.CodeRepresentationOrganizationNotActive, organization},
		{"organization suspended", func(c *domain.RegisterRepresentation) { c.OrganizationStatus = domain.OrganizationSuspended }, domain.CodeRepresentationOrganizationNotActive, organization},
		{"organization offboarded", func(c *domain.RegisterRepresentation) { c.OrganizationStatus = domain.OrganizationOffboarded }, domain.CodeRepresentationOrganizationNotActive, organization},
		{"person not found", func(c *domain.RegisterRepresentation) { c.PersonFound = false }, domain.CodeRepresentationPersonNotFound, person},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := registerRepresentationCommand()
			tt.mutate(&cmd)
			r := domain.NewRepresentation(representationID)
			before := r.Snapshot()

			acc, rej := r.Register(cmd)

			tb.RequireRejected(t, acc, rej, tt.code)
			if got := rej.Details(); !slices.Equal(got, tt.details) {
				t.Fatalf("Details() = %v, want %v", got, tt.details)
			}
			if after := r.Snapshot(); !before.Equal(after) {
				t.Fatalf("snapshot changed on rejection\nbefore: %+v\nafter:  %+v", before, after)
			}
		})
	}
}

func TestRegisterRepresentationReportsThePreconditionsInTheDeclaredOrder(t *testing.T) {
	cmd := domain.RegisterRepresentation{
		OrganizationID:      organizationID,
		PersonID:            personID,
		ValidFrom:           validFrom,
		ValidUntil:          validFrom,
		ScopeMaxAmountMinor: 1,
	}
	want := []kernel.Code{
		domain.CodeRepresentationSourceEvidenceMissing,
		domain.CodeRepresentationValidityInvalid,
		domain.CodeRepresentationScopeInvalid,
		domain.CodeRepresentationOrganizationNotActive,
		domain.CodeRepresentationPersonNotFound,
	}
	fixes := []func(*domain.RegisterRepresentation){
		func(c *domain.RegisterRepresentation) { c.SourceEvidenceID = evidenceID },
		func(c *domain.RegisterRepresentation) { c.ValidUntil = validUntil },
		func(c *domain.RegisterRepresentation) { c.ScopeCurrency = "BRL" },
		func(c *domain.RegisterRepresentation) { c.OrganizationStatus = domain.OrganizationActive },
		func(c *domain.RegisterRepresentation) { c.PersonFound = true },
	}
	for i, code := range want {
		acc, rej := domain.NewRepresentation(representationID).Register(cmd)
		tb.RequireRejected(t, acc, rej, code)
		fixes[i](&cmd)
	}
	if _, rej := domain.NewRepresentation(representationID).Register(cmd); rej != nil {
		t.Fatalf("Register() after every fix rejected with %s", rej.Code())
	}
}

func TestRegisterRepresentationKeepsItsOwnCopyOfTheActTypes(t *testing.T) {
	cmd := registerRepresentationCommand()
	r := domain.NewRepresentation(representationID)
	if _, rej := r.Register(cmd); rej != nil {
		t.Fatalf("Register rejected: %v", rej)
	}

	cmd.ScopeActTypes[0] = "changed by the caller"
	r.Snapshot().ScopeActTypes[1] = "changed through a snapshot"

	if got := r.Snapshot(); !got.Equal(declaredRepresentation()) {
		t.Fatalf("Snapshot() = %+v: a slice held outside reached the aggregate", got)
	}
}

func TestRegisterRepresentationDeterminism(t *testing.T) {
	r1 := domain.NewRepresentation(representationID)
	acc1, _ := r1.Register(registerRepresentationCommand())

	r2 := domain.NewRepresentation(representationID)
	acc2, _ := r2.Register(registerRepresentationCommand())

	if !r1.Snapshot().Equal(r2.Snapshot()) {
		t.Fatal("same inputs must produce the same state")
	}
	tb.RequireSameEvents(t, acc1.Events(), acc2.Events())
}

func TestInForceAtFollowsTheValidity(t *testing.T) {
	bounded := declaredRepresentation()
	indefinite := declaredRepresentation()
	indefinite.ValidUntil = 0
	for _, tc := range []struct {
		name string
		s    domain.RepresentationSnapshot
		at   kernel.Instant
		want bool
	}{
		{"before it starts", bounded, validFrom - 1, false},
		{"the instant it starts", bounded, validFrom, true},
		{"while it lasts", bounded, validFrom + 1, true},
		{"the instant it ends", bounded, validUntil, false},
		{"after it ends", bounded, validUntil + 1, false},
		{"indefinite, long after it starts", indefinite, validUntil * 2, true},
		{"indefinite, before it starts", indefinite, validFrom - 1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.s.InForceAt(tc.at); got != tc.want {
				t.Fatalf("InForceAt(%d) = %v, want %v", tc.at, got, tc.want)
			}
		})
	}
}

func TestFromRepresentationSnapshotRoundTrips(t *testing.T) {
	before := declaredRepresentation()

	got := domain.FromRepresentationSnapshot(before).Snapshot()

	if !got.Equal(before) {
		t.Fatalf("round trip lost state\nbefore: %+v\nafter:  %+v", before, got)
	}
}

func TestRepresentationEqualTellsEveryFieldApart(t *testing.T) {
	base := declaredRepresentation()
	for name, mutate := range map[string]func(*domain.RepresentationSnapshot){
		"id":                  func(s *domain.RepresentationSnapshot) { s.ID = "rep-other" },
		"organization":        func(s *domain.RepresentationSnapshot) { s.OrganizationID = "org-other" },
		"person":              func(s *domain.RepresentationSnapshot) { s.PersonID = "per-other" },
		"power type":          func(s *domain.RepresentationSnapshot) { s.PowerType = domain.PowerOperator },
		"source type":         func(s *domain.RepresentationSnapshot) { s.SourceType = domain.SourceBoardResolution },
		"source evidence":     func(s *domain.RepresentationSnapshot) { s.SourceEvidenceID = "evd-other" },
		"scope currency":      func(s *domain.RepresentationSnapshot) { s.ScopeCurrency = "USD" },
		"scope amount":        func(s *domain.RepresentationSnapshot) { s.ScopeMaxAmountMinor++ },
		"scope act type":      func(s *domain.RepresentationSnapshot) { s.ScopeActTypes[0] = "other" },
		"scope act types":     func(s *domain.RepresentationSnapshot) { s.ScopeActTypes = s.ScopeActTypes[:1] },
		"valid from":          func(s *domain.RepresentationSnapshot) { s.ValidFrom++ },
		"valid until":         func(s *domain.RepresentationSnapshot) { s.ValidUntil++ },
		"verification status": func(s *domain.RepresentationSnapshot) { s.VerificationStatus = domain.RepresentationNew },
		"proposer":            func(s *domain.RepresentationSnapshot) { s.ProposedByUserID = "user-other" },
	} {
		changed := declaredRepresentation()
		mutate(&changed)
		if base.Equal(changed) {
			t.Errorf("Equal ignores the %s", name)
		}
	}
}

func TestEachPowerAndSourceKeepsItsValue(t *testing.T) {
	values := map[string]string{
		string(domain.PowerLegalRepresentative):    "legal_representative",
		string(domain.PowerAttorneyInFact):         "attorney_in_fact",
		string(domain.PowerOperator):               "operator",
		string(domain.SourceArticlesOfAssociation): "articles_of_association",
		string(domain.SourcePowerOfAttorney):       "power_of_attorney",
		string(domain.SourceBoardResolution):       "board_resolution",
		string(domain.SourceInternalDelegation):    "internal_delegation",
	}
	for got, want := range values {
		if got != want {
			t.Errorf("value = %q, want %q", got, want)
		}
	}
}
