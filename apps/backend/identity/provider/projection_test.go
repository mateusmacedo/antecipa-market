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

// WHY: in provider rather than domain because the domain block may not read
// files (depguard), and tb.LoadProjection finds no go.work above a testkit
// pinned in the module cache.
func TestOrganizationsMatchTheProjectionFixture(t *testing.T) {
	raw, err := os.ReadFile("../contract/fixtures/projection/v1/organization.golden")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	f, err := tb.DecodeProjection(raw)
	if err != nil {
		t.Fatalf("decode fixture: %v", err)
	}
	if f.Identity.Context != "identity" || f.Identity.Aggregate != "organization" || len(f.Cases) != 7 {
		t.Fatalf("fixture identity/cases = %+v/%d", f.Identity, len(f.Cases))
	}
	tb.RunProjection(t, f, runOrganization)
}

func runOrganization(t *testing.T, c tb.ProjectionCase) (domainkit.Projection, domainkit.Verdict) {
	t.Helper()
	if c.Command["upr"] != "register" {
		t.Fatalf("unknown upr %q in case %s", c.Command["upr"], c.Name)
	}
	s := registerSubject(domain.RegisterOrganization{
		Kinds:                  kindsFrom(c.Command["kinds"]),
		Jurisdiction:           c.Command["jurisdiction"],
		TaxIDType:              domain.TaxIDType(c.Command["tax_id_type"]),
		TaxID:                  c.Command["tax_id"],
		TaxIDHash:              c.Command["tax_id_hash"],
		TaxIDHashKeyVersion:    int(integer(t, c.Command["tax_id_hash_key_version"])),
		TaxIDAlreadyRegistered: boolean(t, c.Command["tax_id_already_registered"]),
		LegalName:              c.Command["legal_name"],
		TradeName:              c.Command["trade_name"],
		At:                     kernel.Instant(integer(t, c.Command["at"])),
	})
	return domainkit.Run(organizationFromState(t, c.StateBefore), s), domainkit.ReadTwice(organizationFromState(t, c.StateBefore), s)
}

func integer(t *testing.T, s string) int64 {
	t.Helper()
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		t.Fatalf("integer %q: %v", s, err)
	}
	return n
}

func boolean(t *testing.T, s string) bool {
	t.Helper()
	b, err := strconv.ParseBool(s)
	if err != nil {
		t.Fatalf("boolean %q: %v", s, err)
	}
	return b
}

func kindsFrom(listed string) domain.OrganizationKinds {
	if listed == "" {
		return domain.OrganizationKinds{}
	}
	var kinds []domain.OrganizationKind
	for _, name := range strings.Split(listed, ",") {
		kinds = append(kinds, domain.OrganizationKind(name))
	}
	return domain.KindsOf(kinds...)
}

func kindsField(kinds domain.OrganizationKinds) string {
	listed := kinds.List()
	names := make([]string, 0, len(listed))
	for _, kind := range listed {
		names = append(names, string(kind))
	}
	return strings.Join(names, ",")
}

var statusNames = map[domain.OrganizationStatus]string{
	domain.OrganizationNew:        "new",
	domain.OrganizationActive:     "active",
	domain.OrganizationSuspended:  "suspended",
	domain.OrganizationOffboarded: "offboarded",
}

func organizationState(o *domain.Organization) domainkit.Fields {
	s := o.Snapshot()
	return domainkit.Fields{
		"organization":            string(s.ID),
		"kinds":                   kindsField(s.Kinds),
		"jurisdiction":            s.Jurisdiction,
		"tax_id_type":             string(s.TaxIDType),
		"tax_id":                  s.TaxID,
		"tax_id_hash":             s.TaxIDHash,
		"tax_id_hash_key_version": strconv.Itoa(s.TaxIDHashKeyVersion),
		"tax_id_masked":           s.TaxIDMasked,
		"legal_name":              s.LegalName,
		"trade_name":              s.TradeName,
		"status":                  statusNames[s.Status],
		"created_at":              strconv.FormatInt(int64(s.CreatedAt), 10),
	}
}

func organizationFromState(t *testing.T, f domainkit.Fields) *domain.Organization {
	t.Helper()
	s := domain.OrganizationSnapshot{
		ID:                  domain.OrganizationID(f["organization"]),
		Kinds:               kindsFrom(f["kinds"]),
		Jurisdiction:        f["jurisdiction"],
		TaxIDType:           domain.TaxIDType(f["tax_id_type"]),
		TaxID:               f["tax_id"],
		TaxIDHash:           f["tax_id_hash"],
		TaxIDHashKeyVersion: int(integer(t, f["tax_id_hash_key_version"])),
		TaxIDMasked:         f["tax_id_masked"],
		LegalName:           f["legal_name"],
		TradeName:           f["trade_name"],
		CreatedAt:           kernel.Instant(integer(t, f["created_at"])),
	}
	for status, name := range statusNames {
		if name == f["status"] {
			s.Status = status
		}
	}
	return domain.FromOrganizationSnapshot(s)
}

func organizationEvent(e kernel.DomainEvent) (string, domainkit.Fields) {
	registered, ok := e.(domain.OrganizationRegistered)
	if !ok {
		return e.EventName(), domainkit.Fields{}
	}
	return registered.EventName(), domainkit.Fields{
		"organization":  string(registered.OrganizationID),
		"kinds":         kindsField(registered.Kinds),
		"jurisdiction":  registered.Jurisdiction,
		"tax_id_type":   string(registered.TaxIDType),
		"tax_id_hash":   registered.TaxIDHash,
		"tax_id_masked": registered.TaxIDMasked,
		"at":            strconv.FormatInt(int64(registered.At), 10),
	}
}

func registerSubject(cmd domain.RegisterOrganization) domainkit.Subject[*domain.Organization, domain.RegisterOrganizationResponse] {
	return domainkit.Subject[*domain.Organization, domain.RegisterOrganizationResponse]{
		Decide: func(o *domain.Organization) (kernel.Accepted[domain.RegisterOrganizationResponse], *kernel.Rejection) {
			return o.Register(cmd)
		},
		Response: func(r domain.RegisterOrganizationResponse) domainkit.Fields {
			return domainkit.Fields{"organization": string(r.OrganizationID)}
		},
		Event:    organizationEvent,
		Snapshot: organizationState,
		Clone: func(o *domain.Organization) *domain.Organization {
			return domain.FromOrganizationSnapshot(o.Snapshot())
		},
	}
}
