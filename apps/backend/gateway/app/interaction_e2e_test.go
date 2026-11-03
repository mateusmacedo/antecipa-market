//go:build integration

package app_test

import (
	"context"
	"slices"
	"testing"
	"time"

	kernelgrpc "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/execctx"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/app/rpc"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/application"
	servicev1 "github.com/mateusmacedo/antecipa-market/apps/backend/gateway/contract/gen/go/company/gateway/service/v1"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/domain"
)

// internalCall is the execution a consumer of this context mounts: the
// tenant and no subject, which the authorization admits by the tenant alone.
func internalCall(t *testing.T) context.Context {
	t.Helper()
	return execctx.WithExecution(t, context.Background(), execctx.WithoutSubject(), execctx.WithTenant(testTenant))
}

func (e *e2e) published(t *testing.T) []string {
	t.Helper()
	rows, err := e.pool.Query(context.Background(), "SELECT destination || ' ' || message_type FROM outbox ORDER BY id")
	if err != nil {
		t.Fatalf("read outbox: %v", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var row string
		if err := rows.Scan(&row); err != nil {
			t.Fatalf("scan outbox: %v", err)
		}
		out = append(out, row)
	}
	return out
}

// query calls a read of the edge, which takes no idempotency key.
func (e *e2e) query(t *testing.T, method string, req, resp proto.Message) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	ctx = metadata.AppendToOutgoingContext(ctx, kernelgrpc.TenantKey, string(testTenant))
	if err := e.conn.Invoke(ctx, rpc.FullMethod(method), req, resp); err != nil {
		t.Fatalf("%s() = %v", method, err)
	}
}

func TestTheInteractionCycleCommitsWithItsMessagesOverPostgres(t *testing.T) {
	e := serve(t)
	e.ready(t)
	e.command(t, "CreateExternalCommand", create("idem-1", 1), &servicev1.CreateExternalCommandResponse{})
	broken := domain.Mismatch{Kind: domain.MismatchStructure, Detail: "root is not an object", PartnerDeclaredVersion: "2.1"}
	capture := application.RecordExternalInteraction{
		InteractionID: "int-1", ConnectorKey: "prt-1:production", Operation: "submitEffect", Direction: domain.DirectionOutbound,
		Source: domain.InteractionSubmitResponse, ContentType: "application/json", CommandID: "idem-1",
		Request:      domain.RawPayload{Ref: "interactions/prt-1:production/int-1/request", VersionID: "vq", Hash: "hq"},
		Response:     domain.RawPayload{Ref: "interactions/prt-1:production/int-1/response", VersionID: "vs", Hash: "hs"},
		CapturedStep: domain.StepSuspend, Mismatch: broken,
	}

	for range 2 {
		recorded, err := e.service.RecordExternalInteraction(internalCall(t), capture)
		if _, refused := recorded.Rejection(); err != nil || refused || recorded.Response().Status != domain.InteractionStored {
			t.Fatalf("RecordExternalInteraction() = %+v, %v; want stored, and replayed the second time", recorded, err)
		}
	}
	concluded, err := e.service.ConcludeInteraction(internalCall(t), application.ConcludeInteraction{
		InteractionID: "int-1", Revision: 1, Outcome: domain.InteractionSchemaMismatch, Mismatch: broken,
	})
	if _, refused := concluded.Rejection(); err != nil || refused {
		t.Fatalf("ConcludeInteraction() = %+v, %v", concluded, err)
	}

	var reprocessed servicev1.ReprocessInteractionResponse
	e.command(t, "ReprocessInteraction", &servicev1.ReprocessInteractionRequest{InteractionId: "int-1", Justification: "normalizador corrigido"}, &reprocessed)
	if reprocessed.GetReprocessed().GetNormalizationRevision() != 2 {
		t.Fatalf("ReprocessInteraction() = %v, want revision 2", &reprocessed)
	}
	again, err := e.service.ConcludeInteraction(internalCall(t), application.ConcludeInteraction{
		InteractionID: "int-1", Revision: 2, Outcome: domain.InteractionSchemaMismatch, Mismatch: broken,
	})
	if _, refused := again.Rejection(); err != nil || refused {
		t.Fatalf("ConcludeInteraction(revision 2) = %+v, %v", again, err)
	}

	want := []string{
		"gateway.dispatch com.company.gateway.external-command-dispatch-requested.v1",
		"gateway.normalizations com.company.gateway.external-interaction-stored.v1",
		"gateway.events com.company.gateway.external-schema-mismatch-detected.v1",
		"gateway.normalizations com.company.gateway.external-interaction-stored.v1",
		"gateway.events com.company.gateway.external-interaction-reprocessed.v1",
	}
	if got := e.published(t); !slices.Equal(got, want) {
		t.Fatalf("outbox = %v\nwant %v", got, want)
	}

	var byCommand servicev1.FindExternalInteractionsByCommandResponse
	e.query(t, "FindExternalInteractionsByCommand", &servicev1.FindExternalInteractionsByCommandRequest{CommandId: "idem-1"}, &byCommand)
	interactions := byCommand.GetInteractions()
	if len(interactions) != 1 || interactions[0].GetNormalizationRevision() != 2 ||
		interactions[0].GetStatus() != servicev1.InteractionStatus_INTERACTION_STATUS_SCHEMA_MISMATCH ||
		interactions[0].GetMismatchKind() != servicev1.MismatchKind_MISMATCH_KIND_STRUCTURE {
		t.Fatalf("FindExternalInteractionsByCommand() = %v, want the interaction concluded at revision 2", interactions)
	}
}

func TestADuplicateReceptionIsKeptAndTheFirstIsFoundOverPostgres(t *testing.T) {
	e := serve(t)
	callback := application.RecordExternalInteraction{
		InteractionID: "int-1", ConnectorKey: "prt-1:production", Operation: "submitEffect", Direction: domain.DirectionInbound,
		Source: domain.InteractionCallback, ContentType: "application/json", PartnerEventID: "evt-p-1",
		Request: domain.RawPayload{Ref: "interactions/prt-1:production/int-1/request", VersionID: "vq", Hash: "hq"},
	}
	repeated := callback
	repeated.InteractionID, repeated.DuplicateOf = "int-2", "int-1"
	for _, cmd := range []application.RecordExternalInteraction{callback, repeated} {
		if outcome, err := e.service.RecordExternalInteraction(internalCall(t), cmd); err != nil {
			t.Fatalf("RecordExternalInteraction(%s) = %+v, %v", cmd.InteractionID, outcome, err)
		}
	}

	var found servicev1.FindExternalInteractionByPartnerEventResponse
	e.query(t, "FindExternalInteractionByPartnerEvent", &servicev1.FindExternalInteractionByPartnerEventRequest{
		ConnectorId: "prt-1:production", PartnerEventId: "evt-p-1",
	}, &found)
	if found.GetInteraction().GetInteractionId() != "int-1" {
		t.Fatalf("FindExternalInteractionByPartnerEvent() = %v, want the first reception", found.GetInteraction())
	}
	var duplicate servicev1.FindExternalInteractionResponse
	e.query(t, "FindExternalInteraction", &servicev1.FindExternalInteractionRequest{InteractionId: "int-2"}, &duplicate)
	if duplicate.GetInteraction().GetStatus() != servicev1.InteractionStatus_INTERACTION_STATUS_DUPLICATE || duplicate.GetInteraction().GetDuplicateOf() != "int-1" {
		t.Fatalf("FindExternalInteraction(int-2) = %v, want the duplicate of int-1", duplicate.GetInteraction())
	}
	if got := e.published(t); len(got) != 1 {
		t.Fatalf("outbox = %v, want the normalization of the first reception alone", got)
	}
}

func TestALinkedReferenceIsReadBackOverPostgres(t *testing.T) {
	e := serve(t)
	link := &servicev1.LinkExternalReferenceRequest{
		ProviderId: "prt-1", System: servicev1.ExternalSystem_EXTERNAL_SYSTEM_REGISTRY, ReferenceType: servicev1.ReferenceType_REFERENCE_TYPE_ASSET,
		Reference: "dup-0001/a", SubjectType: "asset", SubjectId: "ast-1", AsOf: int64(occurred), ObservedVersion: "v1",
	}
	var linked servicev1.LinkExternalReferenceResponse
	e.command(t, "LinkExternalReference", link, &linked)
	if linked.GetLinked().GetReferenceKey() != "prt-1:registry:asset:dup-0001/a" {
		t.Fatalf("LinkExternalReference() = %v", &linked)
	}

	internal, err := e.service.LinkInteractionReference(internalCall(t), application.LinkInteractionReference{
		Link: application.LinkExternalReference{
			ProviderID: "prt-1", System: domain.SystemRegistry, ReferenceType: domain.ReferenceAsset, Reference: "dup-0001/a",
			SubjectType: "asset", SubjectID: "ast-1", AsOf: occurred + 1, ObservedVersion: "v2",
		},
		InteractionID: "int-1",
	})
	if _, refused := internal.Rejection(); err != nil || refused {
		t.Fatalf("LinkInteractionReference() = %+v, %v; want the renewal under the key derived from the interaction", internal, err)
	}

	var bySubject servicev1.FindExternalReferencesBySubjectResponse
	e.query(t, "FindExternalReferencesBySubject", &servicev1.FindExternalReferencesBySubjectRequest{SubjectType: "asset", SubjectId: "ast-1"}, &bySubject)
	if refs := bySubject.GetReferences(); len(refs) != 1 || refs[0].GetObservedVersion() != "v2" || refs[0].GetNormalizedReference() != "DUP-0001/A" {
		t.Fatalf("FindExternalReferencesBySubject() = %v, want the renewed link", refs)
	}

	link.SubjectId = "ast-2"
	var refused servicev1.LinkExternalReferenceResponse
	e.command(t, "LinkExternalReference", link, &refused)
	if refused.GetRejection().GetCode() != string(domain.CodeExternalReferenceAlreadyLinked) {
		t.Fatalf("LinkExternalReference(ast-2) = %v, want already-linked", &refused)
	}
}
