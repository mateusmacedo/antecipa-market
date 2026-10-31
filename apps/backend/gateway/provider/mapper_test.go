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
