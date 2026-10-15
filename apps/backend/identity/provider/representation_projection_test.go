package provider_test

import (
	"strconv"
	"strings"
	"testing"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/domainkit"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
)

var verificationNames = map[domain.RepresentationStatus]string{
	domain.RepresentationNew:      "new",
	domain.RepresentationDeclared: "declared",
}

func TestRepresentationsMatchTheProjectionFixture(t *testing.T) {
	f := projectionFixture(t, "representation.golden")
	if f.Identity.Context != "identity" || f.Identity.Aggregate != "representation" || len(f.Cases) != 7 {
		t.Fatalf("fixture identity/cases = %+v/%d", f.Identity, len(f.Cases))
	}
	tb.RunProjection(t, f, runRepresentation)
}

func runRepresentation(t *testing.T, c tb.ProjectionCase) (domainkit.Projection, domainkit.Verdict) {
	t.Helper()
	if c.Command["upr"] != "register" {
		t.Fatalf("unknown upr %q in case %s", c.Command["upr"], c.Name)
	}
	s := registerRepresentationSubject(domain.RegisterRepresentation{
		OrganizationID:      domain.OrganizationID(c.Command["organization"]),
		OrganizationStatus:  statusFrom(c.Command["organization_status"]),
		PersonID:            domain.PersonID(c.Command["person"]),
		PersonFound:         boolean(t, c.Command["person_found"]),
		PowerType:           domain.PowerType(c.Command["power_type"]),
		SourceType:          domain.SourceType(c.Command["source_type"]),
		SourceEvidenceID:    c.Command["source_evidence"],
		ScopeCurrency:       c.Command["scope_currency"],
		ScopeMaxAmountMinor: integer(t, c.Command["scope_max_amount_minor"]),
		ScopeActTypes:       listFrom(c.Command["scope_act_types"]),
		ValidFrom:           kernel.Instant(integer(t, c.Command["valid_from"])),
		ValidUntil:          kernel.Instant(integer(t, c.Command["valid_until"])),
		ProposedByUserID:    c.Command["proposed_by"],
	})
	return domainkit.Run(representationFromState(t, c.StateBefore), s), domainkit.ReadTwice(representationFromState(t, c.StateBefore), s)
}

func listFrom(listed string) []string {
	if listed == "" {
		return nil
	}
	return strings.Split(listed, ",")
}

func representationState(r *domain.Representation) domainkit.Fields {
	s := r.Snapshot()
	return domainkit.Fields{
		"representation":         string(s.ID),
		"organization":           string(s.OrganizationID),
		"person":                 string(s.PersonID),
		"power_type":             string(s.PowerType),
		"source_type":            string(s.SourceType),
		"source_evidence":        s.SourceEvidenceID,
		"scope_currency":         s.ScopeCurrency,
		"scope_max_amount_minor": strconv.FormatInt(s.ScopeMaxAmountMinor, 10),
		"scope_act_types":        strings.Join(s.ScopeActTypes, ","),
		"valid_from":             strconv.FormatInt(int64(s.ValidFrom), 10),
		"valid_until":            strconv.FormatInt(int64(s.ValidUntil), 10),
		"verification_status":    verificationNames[s.VerificationStatus],
		"proposed_by":            s.ProposedByUserID,
	}
}

func representationFromState(t *testing.T, f domainkit.Fields) *domain.Representation {
	t.Helper()
	s := domain.RepresentationSnapshot{
		ID:                  domain.RepresentationID(f["representation"]),
		OrganizationID:      domain.OrganizationID(f["organization"]),
		PersonID:            domain.PersonID(f["person"]),
		PowerType:           domain.PowerType(f["power_type"]),
		SourceType:          domain.SourceType(f["source_type"]),
		SourceEvidenceID:    f["source_evidence"],
		ScopeCurrency:       f["scope_currency"],
		ScopeMaxAmountMinor: integer(t, f["scope_max_amount_minor"]),
		ScopeActTypes:       listFrom(f["scope_act_types"]),
		ValidFrom:           kernel.Instant(integer(t, f["valid_from"])),
		ValidUntil:          kernel.Instant(integer(t, f["valid_until"])),
		ProposedByUserID:    f["proposed_by"],
	}
	for status, name := range verificationNames {
		if name == f["verification_status"] {
			s.VerificationStatus = status
		}
	}
	return domain.FromRepresentationSnapshot(s)
}

func registerRepresentationSubject(cmd domain.RegisterRepresentation) domainkit.Subject[*domain.Representation, domain.RegisterRepresentationResponse] {
	return domainkit.Subject[*domain.Representation, domain.RegisterRepresentationResponse]{
		Decide: func(r *domain.Representation) (kernel.Accepted[domain.RegisterRepresentationResponse], *kernel.Rejection) {
			return r.Register(cmd)
		},
		Response: func(r domain.RegisterRepresentationResponse) domainkit.Fields {
			return domainkit.Fields{"representation": string(r.RepresentationID)}
		},
		Event: func(e kernel.DomainEvent) (string, domainkit.Fields) {
			return e.EventName(), domainkit.Fields{}
		},
		Snapshot: representationState,
		Clone: func(r *domain.Representation) *domain.Representation {
			return domain.FromRepresentationSnapshot(r.Snapshot())
		},
	}
}
