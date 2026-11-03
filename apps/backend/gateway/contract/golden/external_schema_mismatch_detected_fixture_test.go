package golden

import (
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/golden"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
	"google.golang.org/protobuf/proto"

	eventv1 "github.com/mateusmacedo/antecipa-market/apps/backend/gateway/contract/gen/go/company/gateway/event/v1"
)

var externalSchemaMismatchDetectedSpec = tb.Spec[proto.Message]{
	Path:    contractRoot + "fixtures/event/v1/external-schema-mismatch-detected.golden",
	Source:  fixtureSource,
	Subject: "int-1001",
	Identity: golden.Identity{
		Fixture:    "gateway/event/v1/external-schema-mismatch-detected",
		Contract:   golden.Contract{Package: "company.gateway.event.v1", Message: "ExternalSchemaMismatchDetected"},
		Type:       "com.company.gateway.external-schema-mismatch-detected.v1",
		DataSchema: "type.googleapis.com/company.gateway.event.v1.ExternalSchemaMismatchDetected",
	},
	Unknown: unknownFieldNumber,
	Enum: tb.Enum{Field: "mismatch_kind", Values: []string{
		"MISMATCH_KIND_STRUCTURE", "MISMATCH_KIND_MISSING_FIELD", "MISMATCH_KIND_TYPE", "MISMATCH_KIND_VERSION",
	}},
	New:        func() proto.Message { return &eventv1.ExternalSchemaMismatchDetected{} },
	FromFields: func(fields map[string]string) (proto.Message, error) { return externalSchemaMismatchDetected(fields) },
	Build: leanFixture(
		externalSchemaMismatchDetectedFields("int-1001", "prt-1:production", "prt-1", "ENVIRONMENT_PRODUCTION", "submitEffect", "duplicata",
			"prt-1:production:submitEffect:eff-1", "MISMATCH_KIND_STRUCTURE", "root is not an object", "2.1", "2026-09-02T12:00:00Z"),
		externalSchemaMismatchDetectedFields("int-1002", "prt-1:production", "prt-1", "ENVIRONMENT_PRODUCTION", "queryAsset", "",
			"", "MISMATCH_KIND_MISSING_FIELD", "campo obrigatório ausente: titular", "", "2026-09-02T12:00:01Z"),
		externalSchemaMismatchDetectedFields("int-2001", "prt-2:homologation", "prt-2", "ENVIRONMENT_HOMOLOGATION", "submitEffect", "cartao",
			"prt-2:homologation:submitEffect:eff-3", "MISMATCH_KIND_TYPE", "amount is a string", "1.0", "2026-09-02T12:00:02Z"),
		externalSchemaMismatchDetectedFields("int-2002", "prt-2:integration", "prt-2", "ENVIRONMENT_INTEGRATION", "requestRelease", "",
			"prt-2:integration:requestRelease:eff-4", "MISMATCH_KIND_VERSION", "schema 3 is not supported", "3.0", "2026-09-02T12:00:03.123456789Z"),
	),
	WantCases:          2,
	WantDiscriminators: 2,
}

func externalSchemaMismatchDetectedFields(interactionID, connectorID, partnerID, environment, operation, assetType, commandID, kind, detail, declaredVersion, at string) map[string]string {
	return map[string]string{
		"interaction_id":           interactionID,
		"connector_id":             connectorID,
		"connector_key":            connectorID,
		"partner_id":               partnerID,
		"environment":              environment,
		"operation":                operation,
		"asset_type":               assetType,
		"command_id":               commandID,
		"mismatch_kind":            kind,
		"mismatch_detail":          detail,
		"partner_declared_version": declaredVersion,
		"at":                       at,
	}
}

func externalSchemaMismatchDetected(fields map[string]string) (*eventv1.ExternalSchemaMismatchDetected, error) {
	environment, err := enumOf(eventv1.Environment_value, fields, "environment")
	if err != nil {
		return nil, err
	}
	kind, err := enumOf(eventv1.MismatchKind_value, fields, "mismatch_kind")
	if err != nil {
		return nil, err
	}
	at, err := parseInstant(fields, "at")
	if err != nil {
		return nil, err
	}
	return &eventv1.ExternalSchemaMismatchDetected{
		InteractionId:          fields["interaction_id"],
		ConnectorId:            fields["connector_id"],
		ConnectorKey:           fields["connector_key"],
		PartnerId:              fields["partner_id"],
		Environment:            eventv1.Environment(environment),
		Operation:              fields["operation"],
		AssetType:              fields["asset_type"],
		CommandId:              fields["command_id"],
		MismatchKind:           eventv1.MismatchKind(kind),
		MismatchDetail:         fields["mismatch_detail"],
		PartnerDeclaredVersion: fields["partner_declared_version"],
		At:                     at,
	}, nil
}
