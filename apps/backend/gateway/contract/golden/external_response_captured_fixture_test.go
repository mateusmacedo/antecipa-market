package golden

import (
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/golden"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
	"google.golang.org/protobuf/proto"

	eventv1 "github.com/mateusmacedo/antecipa-market/apps/backend/gateway/contract/gen/go/company/gateway/event/v1"
)

var externalResponseCapturedSpec = tb.Spec[proto.Message]{
	Path:    contractRoot + "fixtures/event/v1/external-response-captured.golden",
	Source:  fixtureSource,
	Subject: "int-1001",
	Identity: golden.Identity{
		Fixture:    "gateway/event/v1/external-response-captured",
		Contract:   golden.Contract{Package: "company.gateway.event.v1", Message: "ExternalResponseCaptured"},
		Type:       "com.company.gateway.external-response-captured.v1",
		DataSchema: "type.googleapis.com/company.gateway.event.v1.ExternalResponseCaptured",
	},
	Unknown: unknownFieldNumber,
	Enum: tb.Enum{Field: "step", Values: []string{
		"STEP_CONFIRM", "STEP_AWAIT", "STEP_RETRY", "STEP_FAIL", "STEP_QUERY", "STEP_RECONCILE", "STEP_SUSPEND",
	}},
	New:        func() proto.Message { return &eventv1.ExternalResponseCaptured{} },
	FromFields: func(fields map[string]string) (proto.Message, error) { return externalResponseCaptured(fields) },
	Build: withEnumCoverage(
		leanFixture(
			externalResponseCapturedFields("int-1001", "prt-1:production:submitEffect:eff-1", "prt-1:production", "submitEffect", "1",
				"STEP_SUSPEND", "MISMATCH_KIND_STRUCTURE", "root is not an object", "2.1", "2026-09-02T12:00:00Z"),
			externalResponseCapturedFields("int-1002", "prt-1:production:submitEffect:eff-2", "prt-1:production", "submitEffect", "2",
				"STEP_CONFIRM", "MISMATCH_KIND_UNSPECIFIED", "", "", "2026-09-02T12:00:01Z"),
			externalResponseCapturedFields("int-2001", "prt-2:production:requestRelease:eff-3", "prt-2:production", "requestRelease", "3",
				"STEP_AWAIT", "MISMATCH_KIND_UNSPECIFIED", "", "", "2026-09-02T12:00:02Z"),
			externalResponseCapturedFields("int-2002", "prt-2:homologation:submitEffect:eff-4", "prt-2:homologation", "submitEffect", "4",
				"STEP_FAIL", "MISMATCH_KIND_UNSPECIFIED", "", "", "2026-09-02T12:00:03.123456789Z"),
		),
		externalResponseCapturedFields("int-3001", "prt-3:production:submitEffect:eff-5", "prt-3:production", "submitEffect", "5",
			"STEP_RETRY", "MISMATCH_KIND_UNSPECIFIED", "", "", "2026-09-02T12:00:04Z"),
		externalResponseCapturedFields("int-3002", "prt-3:production:submitEffect:eff-6", "prt-3:production", "submitEffect", "6",
			"STEP_QUERY", "MISMATCH_KIND_UNSPECIFIED", "", "", "2026-09-02T12:00:05Z"),
		externalResponseCapturedFields("int-3003", "prt-3:production:submitEffect:eff-7", "prt-3:production", "submitEffect", "7",
			"STEP_RECONCILE", "MISMATCH_KIND_UNSPECIFIED", "", "", "2026-09-02T12:00:06Z"),
	),
	WantCases:          5,
	WantDiscriminators: 2,
}

// externalResponseCapturedFields names the raw payloads after the interaction,
// as the dispatch writes them: interactions/<connectorId>/<interactionId>/...
func externalResponseCapturedFields(interactionID, commandID, connectorID, operation, version, step, kind, detail, declaredVersion, at string) map[string]string {
	prefix := "interactions/" + connectorID + "/" + interactionID + "/"
	return map[string]string{
		"interaction_id":           interactionID,
		"command_id":               commandID,
		"connector_id":             connectorID,
		"operation":                operation,
		"request_ref":              prefix + "request",
		"request_version_id":       "vreq-" + version,
		"request_hash":             "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a0" + version,
		"response_ref":             prefix + "response",
		"response_version_id":      "vres-" + version,
		"response_hash":            "60303ae22b998861bce3b28f33eec1be758a213c86c93c076dbe9f558c11c75" + version,
		"step":                     step,
		"mismatch_kind":            kind,
		"mismatch_detail":          detail,
		"partner_declared_version": declaredVersion,
		"at":                       at,
	}
}

func externalResponseCaptured(fields map[string]string) (*eventv1.ExternalResponseCaptured, error) {
	step, err := enumOf(eventv1.Step_value, fields, "step")
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
	return &eventv1.ExternalResponseCaptured{
		InteractionId:          fields["interaction_id"],
		CommandId:              fields["command_id"],
		ConnectorId:            fields["connector_id"],
		Operation:              fields["operation"],
		RequestRef:             fields["request_ref"],
		RequestVersionId:       fields["request_version_id"],
		RequestHash:            fields["request_hash"],
		ResponseRef:            fields["response_ref"],
		ResponseVersionId:      fields["response_version_id"],
		ResponseHash:           fields["response_hash"],
		Step:                   eventv1.Step(step),
		MismatchKind:           eventv1.MismatchKind(kind),
		MismatchDetail:         fields["mismatch_detail"],
		PartnerDeclaredVersion: fields["partner_declared_version"],
		At:                     at,
	}, nil
}
