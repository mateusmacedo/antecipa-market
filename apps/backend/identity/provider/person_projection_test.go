package provider_test

import (
	"strconv"
	"testing"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/domainkit"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
)

func TestPersonsMatchTheProjectionFixture(t *testing.T) {
	f := projectionFixture(t, "person.golden")
	if f.Identity.Context != "identity" || f.Identity.Aggregate != "person" || len(f.Cases) != 3 {
		t.Fatalf("fixture identity/cases = %+v/%d", f.Identity, len(f.Cases))
	}
	tb.RunProjection(t, f, runPerson)
}

func TestNoProjectedPersonStateHoldsTheDocument(t *testing.T) {
	for _, c := range projectionFixture(t, "person.golden").Cases {
		document := c.Command["document"]
		if domain.NormalizeDocument(document) == "" {
			continue
		}
		for field, value := range c.Expected.StateAfter {
			if value == document || value == domain.NormalizeDocument(document) {
				t.Fatalf("%s: state field %s holds the document, want only its hash and mask (spec note ‡‡)", c.Name, field)
			}
		}
	}
}

func runPerson(t *testing.T, c tb.ProjectionCase) (domainkit.Projection, domainkit.Verdict) {
	t.Helper()
	if c.Command["upr"] != "register" {
		t.Fatalf("unknown upr %q in case %s", c.Command["upr"], c.Name)
	}
	s := registerPersonSubject(domain.RegisterPerson{
		FullName:               c.Command["full_name"],
		Document:               c.Command["document"],
		DocumentHash:           c.Command["document_hash"],
		DocumentHashKeyVersion: int(integer(t, c.Command["document_hash_key_version"])),
		At:                     kernel.Instant(integer(t, c.Command["at"])),
	})
	return domainkit.Run(personFromState(t, c.StateBefore), s), domainkit.ReadTwice(personFromState(t, c.StateBefore), s)
}

func personState(p *domain.Person) domainkit.Fields {
	s := p.Snapshot()
	return domainkit.Fields{
		"person":                    string(s.ID),
		"full_name":                 s.FullName,
		"document_hash":             s.DocumentHash,
		"document_hash_key_version": strconv.Itoa(s.DocumentHashKeyVersion),
		"document_masked":           s.DocumentMasked,
		"created_at":                strconv.FormatInt(int64(s.CreatedAt), 10),
	}
}

func personFromState(t *testing.T, f domainkit.Fields) *domain.Person {
	t.Helper()
	return domain.FromPersonSnapshot(domain.PersonSnapshot{
		ID:                     domain.PersonID(f["person"]),
		FullName:               f["full_name"],
		DocumentHash:           f["document_hash"],
		DocumentHashKeyVersion: int(integer(t, f["document_hash_key_version"])),
		DocumentMasked:         f["document_masked"],
		CreatedAt:              kernel.Instant(integer(t, f["created_at"])),
	})
}

func registerPersonSubject(cmd domain.RegisterPerson) domainkit.Subject[*domain.Person, domain.RegisterPersonResponse] {
	return domainkit.Subject[*domain.Person, domain.RegisterPersonResponse]{
		Decide: func(p *domain.Person) (kernel.Accepted[domain.RegisterPersonResponse], *kernel.Rejection) {
			return p.Register(cmd)
		},
		Response: func(r domain.RegisterPersonResponse) domainkit.Fields {
			return domainkit.Fields{"person": string(r.PersonID)}
		},
		Event: func(e kernel.DomainEvent) (string, domainkit.Fields) {
			return e.EventName(), domainkit.Fields{}
		},
		Snapshot: personState,
		Clone: func(p *domain.Person) *domain.Person {
			return domain.FromPersonSnapshot(p.Snapshot())
		},
	}
}
