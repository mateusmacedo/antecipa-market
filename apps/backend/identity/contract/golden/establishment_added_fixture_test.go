package golden

import (
	"fmt"
	"strconv"

	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/golden"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
	"google.golang.org/protobuf/proto"

	eventv1 "github.com/mateusmacedo/antecipa-market/apps/backend/identity/contract/gen/go/company/identity/event/v1"
)

// The non-canonical case keeps receiver false: the kit's non-canonical encoder ranges only
// populated fields and has no bool arm (testkit v1.0.0-rc.4 tb/golden.go:180-212).
var establishmentAddedSpec = tb.Spec[proto.Message]{
	Path:    contractRoot + "fixtures/event/v1/establishment-added.golden",
	Source:  fixtureSource,
	Subject: "est-1001",
	Identity: golden.Identity{
		Fixture:    "identity/event/v1/establishment-added",
		Contract:   golden.Contract{Package: "company.identity.event.v1", Message: "EstablishmentAdded"},
		Type:       "com.company.identity.establishment-added.v1",
		DataSchema: "type.googleapis.com/company.identity.event.v1.EstablishmentAdded",
	},
	Unknown:    unknownFieldNumber,
	Enum:       tb.Enum{Field: "tax_id_type", Values: []string{"TAX_ID_TYPE_CNPJ", "TAX_ID_TYPE_CPF"}},
	New:        func() proto.Message { return &eventv1.EstablishmentAdded{} },
	FromFields: func(fields map[string]string) (proto.Message, error) { return establishmentAdded(fields) },
	Build: leanFixture(
		establishmentAddedFields("est-1001", "org-1001", "TAX_ID_TYPE_CNPJ",
			"9c1185a5c5e9fc54612808977ee8f548b2258d31b6e2e9a4e5a3ab4c2c4e0f11", "true", "2026-09-02T12:00:00Z"),
		establishmentAddedFields("est-1002", "org-1001", "TAX_ID_TYPE_CPF",
			"5b2d0f0e4c8f1a9e7d36c2b4a1f0e9d8c7b6a5f4e3d2c1b0a9f8e7d6c5b4a392", "false", "2026-09-02T12:00:01Z"),
		establishmentAddedFields("est-2001", "org-2001", "TAX_ID_TYPE_CNPJ",
			"e7a1c3b5d7f9e1a3c5b7d9f1e3a5c7b9d1f3e5a7c9b1d3f5e7a9c1b3d5f7e901", "true", "2026-09-02T12:00:02Z"),
		establishmentAddedFields("est-2002", "org-2001", "TAX_ID_TYPE_CPF",
			"0f1e2d3c4b5a69788796a5b4c3d2e1f00f1e2d3c4b5a69788796a5b4c3d2e1f0", "false", "2026-09-02T12:00:03.123456789Z"),
	),
	WantCases:          2,
	WantDiscriminators: 2,
}

func establishmentAddedFields(establishmentID, organizationID, taxIDType, taxIDHash, receiver, addedAt string) map[string]string {
	return map[string]string{
		"establishment_id": establishmentID,
		"organization_id":  organizationID,
		"tax_id_type":      taxIDType,
		"tax_id_hash":      taxIDHash,
		"receiver":         receiver,
		"added_at":         addedAt,
	}
}

func establishmentAdded(fields map[string]string) (*eventv1.EstablishmentAdded, error) {
	taxIDType, ok := eventv1.TaxIdType_value[fields["tax_id_type"]]
	if !ok {
		return nil, fmt.Errorf("tax_id_type %q is not a TaxIdType", fields["tax_id_type"])
	}
	receiver, err := strconv.ParseBool(fields["receiver"])
	if err != nil {
		return nil, fmt.Errorf("receiver %q: %w", fields["receiver"], err)
	}
	addedAt, err := parseInstant(fields, "added_at")
	if err != nil {
		return nil, err
	}
	return &eventv1.EstablishmentAdded{
		EstablishmentId: fields["establishment_id"],
		OrganizationId:  fields["organization_id"],
		TaxIdType:       eventv1.TaxIdType(taxIDType),
		TaxIdHash:       fields["tax_id_hash"],
		Receiver:        receiver,
		AddedAt:         addedAt,
	}, nil
}
