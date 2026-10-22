package provider_test

import (
	"os"
	"strconv"
	"strings"
	"testing"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/domainkit"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
)

func TestEstablishmentsMatchTheProjectionFixture(t *testing.T) {
	f := projectionFixture(t, "establishment.golden")
	if f.Identity.Context != "identity" || f.Identity.Aggregate != "establishment" || len(f.Cases) != 5 {
		t.Fatalf("fixture identity/cases = %+v/%d", f.Identity, len(f.Cases))
	}
	tb.RunProjection(t, f, runEstablishment)
}

func projectionFixture(t *testing.T, name string) tb.ProjectionFixture {
	t.Helper()
	raw, err := os.ReadFile("../contract/fixtures/projection/v1/" + name)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	f, err := tb.DecodeProjection(raw)
	if err != nil {
		t.Fatalf("decode fixture: %v", err)
	}
	return f
}

func runEstablishment(t *testing.T, c tb.ProjectionCase) (domainkit.Projection, domainkit.Verdict) {
	t.Helper()
	if c.Command["upr"] != "add" {
		t.Fatalf("unknown upr %q in case %s", c.Command["upr"], c.Name)
	}
	s := addSubject(domain.AddEstablishment{
		OrganizationID:      domain.OrganizationID(c.Command["organization"]),
		OrganizationStatus:  statusFrom(c.Command["organization_status"]),
		TaxIDType:           domain.TaxIDType(c.Command["tax_id_type"]),
		TaxID:               c.Command["tax_id"],
		TaxIDHash:           c.Command["tax_id_hash"],
		TaxIDHashKeyVersion: int(integer(t, c.Command["tax_id_hash_key_version"])),
		TradeName:           c.Command["trade_name"],
		Receiver:            boolean(t, c.Command["receiver"]),
		ExternalReferences:  referencesFrom(t, c.Command["external_references"]),
		At:                  kernel.Instant(integer(t, c.Command["at"])),
	})
	before := establishmentFromState(t, c.StateBefore)
	return domainkit.Run(before, s), domainkit.ReadTwice(establishmentFromState(t, c.StateBefore), s)
}

func statusFrom(name string) domain.OrganizationStatus {
	for status, known := range statusNames {
		if known == name {
			return status
		}
	}
	return domain.OrganizationNew
}

// referencesFrom reads the list as provider|system|type|reference|as_of|version
// entries joined by ';', because the golden fixture format holds only string
// scalars.
func referencesFrom(t *testing.T, listed string) []domain.ExternalReference {
	t.Helper()
	if listed == "" {
		return nil
	}
	var refs []domain.ExternalReference
	for _, entry := range strings.Split(listed, ";") {
		parts := strings.Split(entry, "|")
		if len(parts) != 6 {
			t.Fatalf("external reference %q: want six fields", entry)
		}
		refs = append(refs, domain.ExternalReference{
			ProviderID:      parts[0],
			System:          domain.ExternalSystem(parts[1]),
			ReferenceType:   domain.ExternalReferenceType(parts[2]),
			Reference:       parts[3],
			AsOf:            kernel.Instant(integer(t, parts[4])),
			ObservedVersion: parts[5],
		})
	}
	return refs
}

func referencesField(refs []domain.ExternalReference) string {
	entries := make([]string, 0, len(refs))
	for _, r := range refs {
		entries = append(entries, strings.Join([]string{
			r.ProviderID, string(r.System), string(r.ReferenceType), r.Reference, strconv.FormatInt(int64(r.AsOf), 10), r.ObservedVersion,
		}, "|"))
	}
	return strings.Join(entries, ";")
}

func establishmentState(e *domain.Establishment) domainkit.Fields {
	s := e.Snapshot()
	return domainkit.Fields{
		"establishment":           string(s.ID),
		"organization":            string(s.OrganizationID),
		"tax_id_type":             string(s.TaxIDType),
		"tax_id":                  s.TaxID,
		"tax_id_hash":             s.TaxIDHash,
		"tax_id_hash_key_version": strconv.Itoa(s.TaxIDHashKeyVersion),
		"tax_id_masked":           s.TaxIDMasked,
		"trade_name":              s.TradeName,
		"receiver":                strconv.FormatBool(s.Receiver),
		"external_references":     referencesField(s.ExternalReferences),
		"created_at":              strconv.FormatInt(int64(s.CreatedAt), 10),
	}
}

func establishmentFromState(t *testing.T, f domainkit.Fields) *domain.Establishment {
	t.Helper()
	return domain.FromEstablishmentSnapshot(domain.EstablishmentSnapshot{
		ID:                  domain.EstablishmentID(f["establishment"]),
		OrganizationID:      domain.OrganizationID(f["organization"]),
		TaxIDType:           domain.TaxIDType(f["tax_id_type"]),
		TaxID:               f["tax_id"],
		TaxIDHash:           f["tax_id_hash"],
		TaxIDHashKeyVersion: int(integer(t, f["tax_id_hash_key_version"])),
		TaxIDMasked:         f["tax_id_masked"],
		TradeName:           f["trade_name"],
		Receiver:            boolean(t, f["receiver"]),
		ExternalReferences:  referencesFrom(t, f["external_references"]),
		CreatedAt:           kernel.Instant(integer(t, f["created_at"])),
	})
}

func establishmentEvent(e kernel.DomainEvent) (string, domainkit.Fields) {
	added, ok := e.(domain.EstablishmentAdded)
	if !ok {
		return e.EventName(), domainkit.Fields{}
	}
	return added.EventName(), domainkit.Fields{
		"establishment": string(added.EstablishmentID),
		"organization":  string(added.OrganizationID),
		"tax_id_type":   string(added.TaxIDType),
		"tax_id_hash":   added.TaxIDHash,
		"receiver":      strconv.FormatBool(added.Receiver),
		"at":            strconv.FormatInt(int64(added.At), 10),
	}
}

func addSubject(cmd domain.AddEstablishment) domainkit.Subject[*domain.Establishment, domain.AddEstablishmentResponse] {
	return domainkit.Subject[*domain.Establishment, domain.AddEstablishmentResponse]{
		Decide: func(e *domain.Establishment) (kernel.Accepted[domain.AddEstablishmentResponse], *kernel.Rejection) {
			return e.Add(cmd)
		},
		Response: func(r domain.AddEstablishmentResponse) domainkit.Fields {
			return domainkit.Fields{"establishment": string(r.EstablishmentID)}
		},
		Event:    establishmentEvent,
		Snapshot: establishmentState,
		Clone: func(e *domain.Establishment) *domain.Establishment {
			return domain.FromEstablishmentSnapshot(e.Snapshot())
		},
	}
}
