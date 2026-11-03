package rpc

import (
	"context"

	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	kernelgrpc "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/application"
	servicev1 "github.com/mateusmacedo/antecipa-market/apps/backend/gateway/contract/gen/go/company/gateway/service/v1"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/domain"
)

const (
	maxReferenceRunes    = 256
	maxReferenceKeyRunes = 512
	maxVersionRunes      = 128
)

func (s Server) ReprocessInteraction(ctx context.Context, req *servicev1.ReprocessInteractionRequest) (*servicev1.ReprocessInteractionResponse, error) {
	if err := requireID("interaction_id", req.GetInteractionId()); err != nil {
		return nil, err
	}
	if err := requireText("justification", req.GetJustification(), maxTextRunes); err != nil {
		return nil, err
	}
	return kernelgrpc.Command(ctx,
		func(ctx context.Context) (usecase.Outcome[domain.ReprocessResponse], error) {
			return s.Service.ReprocessInteraction(ctx, application.ReprocessInteraction{
				InteractionID: domain.InteractionID(req.GetInteractionId()), Justification: req.GetJustification(),
			})
		},
		func(r *kernel.Rejection) *servicev1.ReprocessInteractionResponse {
			return &servicev1.ReprocessInteractionResponse{Result: &servicev1.ReprocessInteractionResponse_Rejection{Rejection: rejectionOf(r)}}
		},
		func(c domain.ReprocessResponse) *servicev1.ReprocessInteractionResponse {
			return &servicev1.ReprocessInteractionResponse{Result: &servicev1.ReprocessInteractionResponse_Reprocessed{
				Reprocessed: &servicev1.ReprocessedInteraction{InteractionId: string(c.InteractionID), NormalizationRevision: int32(c.NormalizationRevision)},
			}}
		})
}

func (s Server) LinkExternalReference(ctx context.Context, req *servicev1.LinkExternalReferenceRequest) (*servicev1.LinkExternalReferenceResponse, error) {
	cmd, err := linkOf(req)
	if err != nil {
		return nil, err
	}
	return kernelgrpc.Command(ctx,
		func(ctx context.Context) (usecase.Outcome[domain.ReferenceResponse], error) {
			return s.Service.LinkExternalReference(ctx, cmd)
		},
		func(r *kernel.Rejection) *servicev1.LinkExternalReferenceResponse {
			return &servicev1.LinkExternalReferenceResponse{Result: &servicev1.LinkExternalReferenceResponse_Rejection{Rejection: rejectionOf(r)}}
		},
		func(c domain.ReferenceResponse) *servicev1.LinkExternalReferenceResponse {
			return &servicev1.LinkExternalReferenceResponse{Result: &servicev1.LinkExternalReferenceResponse_Linked{
				Linked: &servicev1.LinkedExternalReference{ReferenceKey: string(c.ReferenceKey)},
			}}
		})
}

// linkOf bounds the form only: an empty reference is the domain's refusal,
// with the code the spec declares.
func linkOf(req *servicev1.LinkExternalReferenceRequest) (application.LinkExternalReference, error) {
	var none application.LinkExternalReference
	system, err := enumFromTheWire("system", systemsFromTheWire, req.GetSystem())
	if err != nil {
		return none, err
	}
	kind, err := enumFromTheWire("reference_type", referenceTypesFromTheWire, req.GetReferenceType())
	if err != nil {
		return none, err
	}
	for field, id := range map[string]string{"provider_id": req.GetProviderId(), "subject_type": req.GetSubjectType(), "subject_id": req.GetSubjectId()} {
		if err := requireID(field, id); err != nil {
			return none, err
		}
	}
	for field, text := range map[string]string{"reference": req.GetReference(), "normalized_reference": req.GetNormalizedReference()} {
		if err := requireText(field, text, maxReferenceRunes); err != nil {
			return none, err
		}
	}
	if err := requireText("observed_version", req.GetObservedVersion(), maxVersionRunes); err != nil {
		return none, err
	}
	if req.GetAsOf() <= 0 {
		return none, invalid("as_of", "must be a positive instant in nanoseconds")
	}
	return application.LinkExternalReference{
		ProviderID: domain.PartnerID(req.GetProviderId()), System: system, ReferenceType: kind, Reference: req.GetReference(),
		NormalizedReference: req.GetNormalizedReference(), SubjectType: req.GetSubjectType(), SubjectID: domain.SubjectID(req.GetSubjectId()),
		AsOf: kernel.Instant(req.GetAsOf()), ObservedVersion: req.GetObservedVersion(),
	}, nil
}

func (s Server) FindExternalInteraction(ctx context.Context, req *servicev1.FindExternalInteractionRequest) (*servicev1.FindExternalInteractionResponse, error) {
	if err := requireID("interaction_id", req.GetInteractionId()); err != nil {
		return nil, err
	}
	if _, err := kernelgrpc.ExecutionOf(ctx); err != nil {
		return nil, err
	}
	interaction, err := s.Service.FindExternalInteraction(ctx, domain.InteractionID(req.GetInteractionId()))
	if err != nil {
		return nil, kernelgrpc.StatusOf(err)
	}
	return &servicev1.FindExternalInteractionResponse{Interaction: interactionOnTheWire(interaction)}, nil
}

func (s Server) FindExternalInteractionByPartnerEvent(ctx context.Context, req *servicev1.FindExternalInteractionByPartnerEventRequest) (*servicev1.FindExternalInteractionByPartnerEventResponse, error) {
	for field, id := range map[string]string{"connector_id": req.GetConnectorId(), "partner_event_id": req.GetPartnerEventId()} {
		if err := requireID(field, id); err != nil {
			return nil, err
		}
	}
	if _, err := kernelgrpc.ExecutionOf(ctx); err != nil {
		return nil, err
	}
	interaction, err := s.Service.FindExternalInteractionByPartnerEvent(ctx, domain.ConnectorKey(req.GetConnectorId()), req.GetPartnerEventId())
	if err != nil {
		return nil, kernelgrpc.StatusOf(err)
	}
	return &servicev1.FindExternalInteractionByPartnerEventResponse{Interaction: interactionOnTheWire(interaction)}, nil
}

func (s Server) FindExternalInteractionsByCommand(ctx context.Context, req *servicev1.FindExternalInteractionsByCommandRequest) (*servicev1.FindExternalInteractionsByCommandResponse, error) {
	if !idempotencyKeyFormat.MatchString(req.GetCommandId()) {
		return nil, invalid("command_id", "must have 1 to 100 characters of [A-Za-z0-9._-]")
	}
	if _, err := kernelgrpc.ExecutionOf(ctx); err != nil {
		return nil, err
	}
	found, err := s.Service.FindExternalInteractionsByCommand(ctx, domain.IdempotencyKey(req.GetCommandId()))
	if err != nil {
		return nil, kernelgrpc.StatusOf(err)
	}
	return &servicev1.FindExternalInteractionsByCommandResponse{Interactions: onTheWire(found, interactionOnTheWire)}, nil
}

func (s Server) FindExternalReference(ctx context.Context, req *servicev1.FindExternalReferenceRequest) (*servicev1.FindExternalReferenceResponse, error) {
	if req.GetReferenceKey() == "" {
		return nil, invalid("reference_key", "must not be empty")
	}
	if err := requireText("reference_key", req.GetReferenceKey(), maxReferenceKeyRunes); err != nil {
		return nil, err
	}
	if _, err := kernelgrpc.ExecutionOf(ctx); err != nil {
		return nil, err
	}
	reference, err := s.Service.FindExternalReference(ctx, domain.ReferenceKey(req.GetReferenceKey()))
	if err != nil {
		return nil, kernelgrpc.StatusOf(err)
	}
	return &servicev1.FindExternalReferenceResponse{Reference: referenceOnTheWire(reference)}, nil
}

func (s Server) FindExternalReferencesBySubject(ctx context.Context, req *servicev1.FindExternalReferencesBySubjectRequest) (*servicev1.FindExternalReferencesBySubjectResponse, error) {
	for field, id := range map[string]string{"subject_type": req.GetSubjectType(), "subject_id": req.GetSubjectId()} {
		if err := requireID(field, id); err != nil {
			return nil, err
		}
	}
	if _, err := kernelgrpc.ExecutionOf(ctx); err != nil {
		return nil, err
	}
	found, err := s.Service.FindExternalReferencesBySubject(ctx, req.GetSubjectType(), domain.SubjectID(req.GetSubjectId()))
	if err != nil {
		return nil, kernelgrpc.StatusOf(err)
	}
	return &servicev1.FindExternalReferencesBySubjectResponse{References: onTheWire(found, referenceOnTheWire)}, nil
}

var systemsFromTheWire = map[servicev1.ExternalSystem]domain.ExternalSystem{
	servicev1.ExternalSystem_EXTERNAL_SYSTEM_REGISTRY:   domain.SystemRegistry,
	servicev1.ExternalSystem_EXTERNAL_SYSTEM_BOOKKEEPER: domain.SystemBookkeeper,
	servicev1.ExternalSystem_EXTERNAL_SYSTEM_ACQUIRER:   domain.SystemAcquirer,
	servicev1.ExternalSystem_EXTERNAL_SYSTEM_FUNDER:     domain.SystemFunder,
	servicev1.ExternalSystem_EXTERNAL_SYSTEM_DEPOSITORY: domain.SystemDepository,
}

var referenceTypesFromTheWire = map[servicev1.ReferenceType]domain.ReferenceType{
	servicev1.ReferenceType_REFERENCE_TYPE_ASSET:         domain.ReferenceAsset,
	servicev1.ReferenceType_REFERENCE_TYPE_EFFECT:        domain.ReferenceEffect,
	servicev1.ReferenceType_REFERENCE_TYPE_AUTHORIZATION: domain.ReferenceAuthorization,
	servicev1.ReferenceType_REFERENCE_TYPE_COMMAND:       domain.ReferenceCommand,
	servicev1.ReferenceType_REFERENCE_TYPE_SETTLEMENT:    domain.ReferenceSettlement,
}

var directionsToTheWire = map[domain.Direction]servicev1.InteractionDirection{
	domain.DirectionOutbound: servicev1.InteractionDirection_INTERACTION_DIRECTION_OUTBOUND,
	domain.DirectionInbound:  servicev1.InteractionDirection_INTERACTION_DIRECTION_INBOUND,
}

var sourcesToTheWire = map[domain.InteractionSource]servicev1.InteractionSource{
	domain.InteractionSubmitResponse: servicev1.InteractionSource_INTERACTION_SOURCE_SUBMIT_RESPONSE,
	domain.InteractionStateQuery:     servicev1.InteractionSource_INTERACTION_SOURCE_STATE_QUERY,
	domain.InteractionCallback:       servicev1.InteractionSource_INTERACTION_SOURCE_CALLBACK,
	domain.InteractionFile:           servicev1.InteractionSource_INTERACTION_SOURCE_FILE,
	domain.InteractionExternalRead:   servicev1.InteractionSource_INTERACTION_SOURCE_EXTERNAL_READ,
}

var interactionStatusesToTheWire = map[domain.InteractionStatus]servicev1.InteractionStatus{
	domain.InteractionStored:         servicev1.InteractionStatus_INTERACTION_STATUS_STORED,
	domain.InteractionDuplicate:      servicev1.InteractionStatus_INTERACTION_STATUS_DUPLICATE,
	domain.InteractionNormalized:     servicev1.InteractionStatus_INTERACTION_STATUS_NORMALIZED,
	domain.InteractionUncorrelated:   servicev1.InteractionStatus_INTERACTION_STATUS_UNCORRELATED,
	domain.InteractionSchemaMismatch: servicev1.InteractionStatus_INTERACTION_STATUS_SCHEMA_MISMATCH,
}

var findingsToTheWire = map[domain.Finding]servicev1.Finding{
	domain.FindingApplied:      servicev1.Finding_FINDING_APPLIED,
	domain.FindingRejected:     servicev1.Finding_FINDING_REJECTED,
	domain.FindingAbsent:       servicev1.Finding_FINDING_ABSENT,
	domain.FindingPending:      servicev1.Finding_FINDING_PENDING,
	domain.FindingInconclusive: servicev1.Finding_FINDING_INCONCLUSIVE,
}

var mismatchKindsToTheWire = map[domain.MismatchKind]servicev1.MismatchKind{
	domain.MismatchStructure:    servicev1.MismatchKind_MISMATCH_KIND_STRUCTURE,
	domain.MismatchMissingField: servicev1.MismatchKind_MISMATCH_KIND_MISSING_FIELD,
	domain.MismatchType:         servicev1.MismatchKind_MISMATCH_KIND_TYPE,
	domain.MismatchVersion:      servicev1.MismatchKind_MISMATCH_KIND_VERSION,
}

var stepsToTheWire = map[domain.Step]servicev1.Step{
	domain.StepConfirm:   servicev1.Step_STEP_CONFIRM,
	domain.StepAwait:     servicev1.Step_STEP_AWAIT,
	domain.StepRetry:     servicev1.Step_STEP_RETRY,
	domain.StepFail:      servicev1.Step_STEP_FAIL,
	domain.StepQuery:     servicev1.Step_STEP_QUERY,
	domain.StepReconcile: servicev1.Step_STEP_RECONCILE,
	domain.StepSuspend:   servicev1.Step_STEP_SUSPEND,
}

var (
	systemsToTheWire        = toTheWire(systemsFromTheWire)
	referenceTypesToTheWire = toTheWire(referenceTypesFromTheWire)
)

func interactionOnTheWire(s domain.ExternalInteractionSnapshot) *servicev1.ExternalInteraction {
	return &servicev1.ExternalInteraction{
		InteractionId: string(s.ID), ConnectorId: string(s.ConnectorKey), Operation: s.Operation, Direction: directionsToTheWire[s.Direction],
		Source: sourcesToTheWire[s.Source], CommandId: string(s.CommandID), ContentType: s.ContentType, CapturedStep: stepsToTheWire[s.CapturedStep],
		PartnerEventId: s.PartnerEventID, BusinessDate: s.BusinessDate,
		RequestRef: s.Request.Ref, RequestVersionId: s.Request.VersionID, RequestHash: s.Request.Hash,
		ResponseRef: s.Response.Ref, ResponseVersionId: s.Response.VersionID, ResponseHash: s.Response.Hash,
		DuplicateOf: string(s.DuplicateOf), Status: interactionStatusesToTheWire[s.Status], NormalizationRevision: int32(s.NormalizationRevision),
		ReadingHash: s.ReadingHash, Finding: findingsToTheWire[s.Finding], PreviousOutcome: interactionStatusesToTheWire[s.PreviousOutcome],
		PreviousReadingHash: s.PreviousReadingHash, MismatchKind: mismatchKindsToTheWire[s.Mismatch.Kind], MismatchDetail: s.Mismatch.Detail,
		PartnerDeclaredVersion: s.Mismatch.PartnerDeclaredVersion, ReceivedAt: int64(s.ReceivedAt),
	}
}

func referenceOnTheWire(s domain.ExternalReferenceSnapshot) *servicev1.ExternalReference {
	return &servicev1.ExternalReference{
		ReferenceKey: string(s.Key), ProviderId: string(s.ProviderID), System: systemsToTheWire[s.System],
		ReferenceType: referenceTypesToTheWire[s.ReferenceType], Reference: s.Reference, NormalizedReference: s.NormalizedReference,
		SubjectType: s.SubjectType, SubjectId: string(s.SubjectID), AsOf: int64(s.AsOf), ObservedVersion: s.ObservedVersion,
	}
}
