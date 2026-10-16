package golden

import (
	"fmt"
	"strings"

	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/golden"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
	"google.golang.org/protobuf/proto"

	eventv1 "github.com/mateusmacedo/antecipa-market/apps/backend/identity/contract/gen/go/company/identity/event/v1"
)

var organizationRegisteredSpec = tb.Spec[proto.Message]{
	Path:    contractRoot + "fixtures/event/v1/organization-registered.golden",
	Source:  fixtureSource,
	Subject: "org-1001",
	Identity: golden.Identity{
		Fixture:    "identity/event/v1/organization-registered",
		Contract:   golden.Contract{Package: "company.identity.event.v1", Message: "OrganizationRegistered"},
		Type:       "com.company.identity.organization-registered.v1",
		DataSchema: "type.googleapis.com/company.identity.event.v1.OrganizationRegistered",
	},
	Unknown:    unknownFieldNumber,
	Enum:       tb.Enum{Field: "tax_id_type", Values: []string{"TAX_ID_TYPE_CNPJ", "TAX_ID_TYPE_CPF"}},
	New:        func() proto.Message { return &eventv1.OrganizationRegistered{} },
	FromFields: func(fields map[string]string) (proto.Message, error) { return organizationRegistered(fields) },
	Build: leanFixture(
		organizationRegisteredFields("org-1001", "ORGANIZATION_KIND_CLIENT", "BR", "TAX_ID_TYPE_CNPJ",
			"4a455609da3a3d6d689b65257f38532e3868396abc39d3b92311b18489f169dc", "**.***.***/**DE-35", "2026-09-02T12:00:00Z"),
		organizationRegisteredFields("org-1002", "ORGANIZATION_KIND_FUNDER,ORGANIZATION_KIND_PARTNER", "BR", "TAX_ID_TYPE_CPF",
			"accdef5fa5a63d40218f9b3fb5abdfc9eeb4036cbdbe46cff36bff2ec9462a3d", "***.***.*89-09", "2026-09-02T12:00:01Z"),
		organizationRegisteredFields("org-2001", "ORGANIZATION_KIND_PLATFORM", "PT", "TAX_ID_TYPE_CNPJ",
			"2ad689d9d2bdfa27268b09d5f0bd86e01cc7545bfb72a11b4c28450f3098b053", "**.***.***/**01-81", "2026-09-02T12:00:02Z"),
		organizationRegisteredFields("org-2002", "", "BR", "TAX_ID_TYPE_CPF",
			"d9e740ecc0099261fc6176681ef8f152dd08514d121084cef8a2de3d6a2f44d3", "***.***.*21-00", "2026-09-02T12:00:03.123456789Z"),
	),
	WantCases:          2,
	WantDiscriminators: 2,
}

func organizationRegisteredFields(organizationID, kinds, jurisdiction, taxIDType, taxIDHash, taxIDMasked, registeredAt string) map[string]string {
	return map[string]string{
		"organization_id": organizationID,
		"kinds":           kinds,
		"jurisdiction":    jurisdiction,
		"tax_id_type":     taxIDType,
		"tax_id_hash":     taxIDHash,
		"tax_id_masked":   taxIDMasked,
		"registered_at":   registeredAt,
	}
}

func organizationRegistered(fields map[string]string) (*eventv1.OrganizationRegistered, error) {
	kinds, err := organizationKinds(fields["kinds"])
	if err != nil {
		return nil, err
	}
	taxIDType, ok := eventv1.TaxIdType_value[fields["tax_id_type"]]
	if !ok {
		return nil, fmt.Errorf("tax_id_type %q is not a TaxIdType", fields["tax_id_type"])
	}
	registeredAt, err := parseInstant(fields, "registered_at")
	if err != nil {
		return nil, err
	}
	return &eventv1.OrganizationRegistered{
		OrganizationId: fields["organization_id"],
		Kinds:          kinds,
		Jurisdiction:   fields["jurisdiction"],
		TaxIdType:      eventv1.TaxIdType(taxIDType),
		TaxIdHash:      fields["tax_id_hash"],
		TaxIdMasked:    fields["tax_id_masked"],
		RegisteredAt:   registeredAt,
	}, nil
}

// A fixture payload holds only string scalars, as the golden fixture format
// requires.
func organizationKinds(listed string) ([]eventv1.OrganizationKind, error) {
	if listed == "" {
		return nil, nil
	}
	names := strings.Split(listed, ",")
	kinds := make([]eventv1.OrganizationKind, 0, len(names))
	for _, name := range names {
		kind, ok := eventv1.OrganizationKind_value[name]
		if !ok {
			return nil, fmt.Errorf("kinds: %q is not an OrganizationKind", name)
		}
		kinds = append(kinds, eventv1.OrganizationKind(kind))
	}
	return kinds, nil
}
