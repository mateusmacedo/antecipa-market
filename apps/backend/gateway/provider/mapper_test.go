package provider_test

import (
	"errors"
	"testing"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	eventv1 "github.com/mateusmacedo/antecipa-market/apps/backend/gateway/contract/gen/go/company/gateway/event/v1"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/provider"
)

type strayEvent struct{}

func (strayEvent) EventName() string { return "gateway.stray" }

const mapped = kernel.Instant(1_755_432_000_123_456_789)

var stamp = &timestamppb.Timestamp{Seconds: 1_755_432_000, Nanos: 123_456_789}

func header() domain.CommandHeader {
	return domain.CommandHeader{IdempotencyKey: "idem-1", TenantID: "tnt-1", ConnectorKey: "prt-1:production", Operation: "submitEffect", BusinessKey: "eff-1"}
}

func TestTheMapperPublishesEachEventUnderItsContract(t *testing.T) {
	tests := []struct {
		event    kernel.DomainEvent
		want     proto.Message
		wantType string
	}{
		{
			domain.ExternalCommandDispatchRequested{IdempotencyKey: "idem-1", ConnectorKey: "prt-1:production", Operation: "submitEffect", At: mapped},
			&eventv1.ExternalCommandDispatchRequested{IdempotencyKey: "idem-1", ConnectorId: "prt-1:production", Operation: "submitEffect", At: stamp},
			"com.company.gateway.external-command-dispatch-requested.v1",
		},
		{
			domain.ExternalCommandSubmitted{CommandHeader: header(), Submission: 2, At: mapped},
			&eventv1.ExternalCommandSubmitted{IdempotencyKey: "idem-1", TenantId: "tnt-1", ConnectorId: "prt-1:production", Operation: "submitEffect", BusinessKey: "eff-1", Submission: 2, At: stamp},
			"com.company.gateway.external-command-submitted.v1",
		},
		{
			domain.ExternalCommandAcknowledged{CommandHeader: header(), ExternalReferenceID: "ext-1", At: mapped},
			&eventv1.ExternalCommandAcknowledged{IdempotencyKey: "idem-1", TenantId: "tnt-1", ConnectorId: "prt-1:production", Operation: "submitEffect", BusinessKey: "eff-1", ExternalReferenceId: "ext-1", At: stamp},
			"com.company.gateway.external-command-acknowledged.v1",
		},
		{
			domain.ExternalCommandConfirmed{CommandHeader: header(), ExternalReferenceID: "ext-1", ConfirmationSource: domain.SourceHuman, Partial: true, InteractionID: "int-1", ConfirmedAt: mapped},
			&eventv1.ExternalCommandConfirmed{
				IdempotencyKey: "idem-1", TenantId: "tnt-1", ConnectorId: "prt-1:production", Operation: "submitEffect", BusinessKey: "eff-1",
				ExternalReferenceId: "ext-1", ConfirmationSource: eventv1.ConfirmationSource_CONFIRMATION_SOURCE_HUMAN, Partial: true, InteractionId: "int-1", ConfirmedAt: stamp,
			},
			"com.company.gateway.external-command-confirmed.v1",
		},
		{
			domain.ExternalCommandFailed{CommandHeader: header(), Category: domain.CategoryFinalValidation, PartnerCode: "E1", PartnerMessage: "campo inválido", InteractionID: "int-1", At: mapped},
			&eventv1.ExternalCommandFailed{
				IdempotencyKey: "idem-1", TenantId: "tnt-1", ConnectorId: "prt-1:production", Operation: "submitEffect", BusinessKey: "eff-1",
				Category: eventv1.FailureCategory_FAILURE_CATEGORY_FINAL_VALIDATION, PartnerCode: "E1", PartnerMessage: "campo inválido", InteractionId: "int-1", At: stamp,
			},
			"com.company.gateway.external-command-failed.v1",
		},
		{
			domain.ExternalOutcomeUnknown{CommandHeader: header(), FailureCategory: domain.CategoryUnknownSent, Submissions: 1, LastSubmittedAt: mapped, StateQuerySupported: true, RequiresHumanResolution: true, UnknownSince: mapped},
			&eventv1.ExternalOutcomeUnknown{
				IdempotencyKey: "idem-1", TenantId: "tnt-1", ConnectorId: "prt-1:production", Operation: "submitEffect", BusinessKey: "eff-1",
				FailureCategory: eventv1.FailureCategory_FAILURE_CATEGORY_UNKNOWN_SENT, Submissions: 1, LastSubmittedAt: stamp, StateQuerySupported: true,
				RequiresHumanResolution: true, UnknownSince: stamp,
			},
			"com.company.gateway.external-outcome-unknown.v1",
		},
		{
			domain.ExternalCommandAmbiguityExpired{CommandHeader: header(), PartnerID: "prt-1", Status: domain.StatusAcknowledged, WaitingSince: mapped, AmbiguityDeadline: 3600, StateQuerySupported: true, At: mapped},
			&eventv1.ExternalCommandAmbiguityExpired{
				IdempotencyKey: "idem-1", TenantId: "tnt-1", ConnectorId: "prt-1:production", PartnerId: "prt-1", Operation: "submitEffect", BusinessKey: "eff-1",
				Status: eventv1.WaitingStatus_WAITING_STATUS_ACKNOWLEDGED, WaitingSince: stamp, AmbiguityDeadline: 3600, StateQuerySupported: true, At: stamp,
			},
			"com.company.gateway.external-command-ambiguity-expired.v1",
		},
	}
	for _, tc := range tests {
		t.Run(tc.event.EventName(), func(t *testing.T) {
			got, err := provider.Mapper{}.Map(tc.event)

			if err != nil {
				t.Fatalf("Map() = %v", err)
			}
			if !proto.Equal(got.Message, tc.want) || got.Type != tc.wantType {
				t.Fatalf("Map() = %v %q, want %v %q", got.Message, got.Type, tc.want, tc.wantType)
			}
		})
	}
}

func TestTheMapperPublishesEachInteractionMessageUnderItsContract(t *testing.T) {
	request := domain.RawPayload{Ref: "interactions/prt-1:production/int-1/request", VersionID: "vq", Hash: "hq"}
	response := domain.RawPayload{Ref: "interactions/prt-1:production/int-1/response", VersionID: "vs", Hash: "hs"}
	broken := domain.Mismatch{Kind: domain.MismatchVersion, Detail: "schema 3", PartnerDeclaredVersion: "3.0"}
	tests := []struct {
		event    kernel.DomainEvent
		want     proto.Message
		wantType string
	}{
		{
			domain.ExternalResponseCaptured{
				InteractionID: "int-1", CommandID: "idem-1", ConnectorKey: "prt-1:production", Operation: "submitEffect",
				Request: request, Response: response, ContentType: "application/json", Step: domain.StepSuspend, Mismatch: broken, At: mapped,
			},
			&eventv1.ExternalResponseCaptured{
				InteractionId: "int-1", CommandId: "idem-1", ConnectorId: "prt-1:production", Operation: "submitEffect",
				RequestRef: request.Ref, RequestVersionId: "vq", RequestHash: "hq", ResponseRef: response.Ref, ResponseVersionId: "vs", ResponseHash: "hs",
				ContentType: "application/json", Step: eventv1.Step_STEP_SUSPEND, MismatchKind: eventv1.MismatchKind_MISMATCH_KIND_VERSION, MismatchDetail: "schema 3",
				PartnerDeclaredVersion: "3.0", At: stamp,
			},
			"com.company.gateway.external-response-captured.v1",
		},
		{
			domain.ExternalInteractionStored{
				InteractionID: "int-1", ConnectorKey: "prt-1:production", Operation: "submitEffect", Direction: domain.DirectionInbound,
				Source: domain.InteractionCallback, CommandID: "idem-1", NormalizationRevision: 2, At: mapped,
			},
			&eventv1.ExternalInteractionStored{
				InteractionId: "int-1", ConnectorId: "prt-1:production", Operation: "submitEffect",
				Direction: eventv1.InteractionDirection_INTERACTION_DIRECTION_INBOUND, Source: eventv1.InteractionSource_INTERACTION_SOURCE_CALLBACK,
				CommandId: "idem-1", NormalizationRevision: 2, At: stamp,
			},
			"com.company.gateway.external-interaction-stored.v1",
		},
		{
			domain.ExternalSchemaMismatchDetected{
				InteractionID: "int-1", ConnectorKey: "prt-1:production", PartnerID: "prt-1", Environment: domain.EnvironmentProduction,
				Operation: "submitEffect", AssetType: "duplicata", CommandID: "idem-1", Mismatch: broken, At: mapped,
			},
			&eventv1.ExternalSchemaMismatchDetected{
				InteractionId: "int-1", ConnectorId: "prt-1:production", ConnectorKey: "prt-1:production", PartnerId: "prt-1",
				Environment: eventv1.Environment_ENVIRONMENT_PRODUCTION, Operation: "submitEffect", AssetType: "duplicata", CommandId: "idem-1",
				MismatchKind: eventv1.MismatchKind_MISMATCH_KIND_VERSION, MismatchDetail: "schema 3", PartnerDeclaredVersion: "3.0", At: stamp,
			},
			"com.company.gateway.external-schema-mismatch-detected.v1",
		},
		{
			domain.ExternalInteractionUncorrelated{
				InteractionID: "int-1", ConnectorKey: "prt-1:homologation", PartnerID: "prt-1", Environment: domain.EnvironmentHomologation,
				Operation: "submitEffect", PartnerEventID: "evt-p-1", BusinessDate: "2026-09-02", ReceivedAt: mapped, At: mapped,
			},
			&eventv1.ExternalInteractionUncorrelated{
				InteractionId: "int-1", ConnectorId: "prt-1:homologation", ConnectorKey: "prt-1:homologation", PartnerId: "prt-1",
				Environment: eventv1.Environment_ENVIRONMENT_HOMOLOGATION, Operation: "submitEffect", PartnerEventId: "evt-p-1",
				BusinessDate: "2026-09-02", ReceivedAt: stamp, At: stamp,
			},
			"com.company.gateway.external-interaction-uncorrelated.v1",
		},
		{
			domain.ExternalInteractionReprocessed{
				InteractionID: "int-1", ConnectorKey: "prt-1:production", PartnerID: "prt-1", Operation: "submitEffect", CommandID: "idem-1",
				PreviousRevision: 1, NormalizationRevision: 2, PreviousOutcome: domain.InteractionUncorrelated, Outcome: domain.InteractionNormalized,
				OutcomeChanged: true, TerminalCommandAffected: true, At: mapped,
			},
			&eventv1.ExternalInteractionReprocessed{
				InteractionId: "int-1", ConnectorId: "prt-1:production", PartnerId: "prt-1", Operation: "submitEffect", CommandId: "idem-1",
				PreviousRevision: 1, NormalizationRevision: 2, PreviousOutcome: eventv1.InteractionOutcome_INTERACTION_OUTCOME_UNCORRELATED,
				Outcome: eventv1.InteractionOutcome_INTERACTION_OUTCOME_NORMALIZED, OutcomeChanged: true, TerminalCommandAffected: true, At: stamp,
			},
			"com.company.gateway.external-interaction-reprocessed.v1",
		},
	}
	for _, tc := range tests {
		t.Run(tc.event.EventName(), func(t *testing.T) {
			got, err := provider.Mapper{}.Map(tc.event)

			if err != nil {
				t.Fatalf("Map() = %v", err)
			}
			if !proto.Equal(got.Message, tc.want) || got.Type != tc.wantType {
				t.Fatalf("Map() = %v %q, want %v %q", got.Message, got.Type, tc.want, tc.wantType)
			}
		})
	}
}

func TestTheMapperLeavesAnAbsentInstantAbsent(t *testing.T) {
	got, err := provider.Mapper{}.Map(domain.ExternalOutcomeUnknown{CommandHeader: header(), UnknownSince: mapped})

	if err != nil || got.Message.(*eventv1.ExternalOutcomeUnknown).GetLastSubmittedAt() != nil {
		t.Fatalf("Map() = %v, %v; want no last submission instant", got.Message, err)
	}
}

func TestTheMapperRefusesAnEventWithoutContract(t *testing.T) {
	if _, err := (provider.Mapper{}).Map(strayEvent{}); !errors.Is(err, postgres.ErrUnmappedEvent) {
		t.Fatalf("Map(stray) = %v, want ErrUnmappedEvent", err)
	}
}

func TestTheMapperPublishesTheSpentResendBudgetUnderItsWireValue(t *testing.T) {
	got, err := provider.Mapper{}.Map(domain.ExternalCommandFailed{CommandHeader: header(), Category: domain.CategoryResendBudgetExhausted, At: mapped})

	if err != nil || got.Message.(*eventv1.ExternalCommandFailed).GetCategory() != eventv1.FailureCategory_FAILURE_CATEGORY_RESEND_BUDGET_EXHAUSTED {
		t.Fatalf("Map() = %v, %v; want FAILURE_CATEGORY_RESEND_BUDGET_EXHAUSTED", got.Message, err)
	}
}
