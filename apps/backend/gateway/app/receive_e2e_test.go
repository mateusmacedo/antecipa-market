//go:build integration

package app_test

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"

	kernelapp "github.com/mateusmacedo/dmpf/libs/backend/go/app"
	"github.com/mateusmacedo/dmpf/libs/backend/go/memory"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/execctx"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/app"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/appkit"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/application"
	servicev1 "github.com/mateusmacedo/antecipa-market/apps/backend/gateway/contract/gen/go/company/gateway/service/v1"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/ports"
)

// racingStore keeps every version of a key and holds the first writes until
// all of them are in, so the records that follow them race.
type racingStore struct {
	mu      sync.Mutex
	objects map[string][]ports.RawBody
	racers  sync.WaitGroup
}

func newRacingStore(racers int) *racingStore {
	s := &racingStore{objects: map[string][]ports.RawBody{}}
	s.racers.Add(racers)
	return s
}

func (s *racingStore) Put(_ context.Context, key string, body ports.RawBody) (domain.RawPayload, error) {
	s.mu.Lock()
	s.objects[key] = append(s.objects[key], body)
	version := len(s.objects[key])
	s.mu.Unlock()
	s.racers.Done()
	s.racers.Wait()
	return domain.RawPayload{Ref: key, VersionID: "v" + strconv.Itoa(version), Hash: "h-" + strconv.Itoa(len(body.Body))}, nil
}

func (s *racingStore) Get(context.Context, domain.RawPayload) (ports.RawBody, error) {
	return ports.RawBody{}, ports.ErrRawPayloadDefect
}

func (s *racingStore) versions(key string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.objects[key])
}

func receivedWith(t *testing.T, raw ports.RawStore) *e2e {
	t.Helper()
	pool := appkit.OpenPool(t)
	policy, err := kernelapp.IdempotencyPolicy(time.Second, 24*time.Hour)
	if err != nil {
		t.Fatalf("IdempotencyPolicy() = %v", err)
	}
	service := app.NewService(pool, memory.FixedClock{At: occurred}, &memory.SequenceIDs{Prefix: "e2e-"}, app.Waits{Message: time.Second, Command: 5 * time.Second})
	service.Idempotency, service.Raw = policy, raw
	e := &e2e{pool: pool, service: service, conn: serveOver(t, service)}
	var declared servicev1.DeclareCapabilityProfileResponse
	e.command(t, "DeclareCapabilityProfile", &servicev1.DeclareCapabilityProfileRequest{
		ConnectorId: "prt-1:production", Version: 1, EffectiveFrom: int64(occurred),
		Operations: []*servicev1.OperationCapability{{
			Operation: "submitEffect", Supported: true, IdempotencySupport: servicev1.IdempotencySupport_IDEMPOTENCY_SUPPORT_KEY_HEADER,
			Efficacy: servicev1.Efficacy_EFFICACY_ON_SUBMIT, Quiescence: 1, MaxQueries: 3, KeyAlphabet: "0123456789abcdef",
			KeyMaxLength: 32, AmbiguityDeadline: 3600, MaxSubmissions: 3, MaxPayloadBytes: 1 << 12, Protocol: servicev1.Protocol_PROTOCOL_ASYNC_CALLBACK,
			EncumbranceCoverage: servicev1.EncumbranceCoverage_ENCUMBRANCE_COVERAGE_UNAVAILABLE,
		}},
	}, &declared)
	if declared.GetDeclared() == nil {
		t.Fatalf("DeclareCapabilityProfile() = %v", &declared)
	}
	return e
}

// WHY: decision 19 deduplicates without a read before the save, so only a
// real concurrent insert proves the loser is kept, as a duplicate, and never
// normalized.
func TestTwoReceiptsOfOnePartnerEventRaceOnThePrimaryKeyOverPostgres(t *testing.T) {
	raw := newRacingStore(2)
	e := receivedWith(t, raw)
	receipt := application.ReceivePartnerInteraction{
		ConnectorKey: "prt-1:production", Operation: "submitEffect", Source: domain.InteractionCallback, PartnerEventID: "evt-p-1",
		ContentType: "application/json", Body: []byte(`{"format":"partnersim/v1","status":200}`),
	}
	contexts := make([]context.Context, 2)
	for i := range contexts {
		contexts[i] = execctx.WithKey(t, context.Background(), fmt.Sprintf("rcv-%d", i), execctx.WithoutSubject(), execctx.WithTenant(testTenant))
	}

	responses, failures := make([]domain.InteractionResponse, 2), make([]error, 2)
	var receipts sync.WaitGroup
	for i, ctx := range contexts {
		receipts.Go(func() {
			outcome, err := e.service.ReceivePartnerInteraction(ctx, receipt)
			if _, refused := outcome.Rejection(); refused && err == nil {
				err = fmt.Errorf("refused: %+v", outcome)
			}
			responses[i], failures[i] = outcome.Response(), err
		})
	}
	receipts.Wait()

	first, err := e.service.DerivedKeys.InboundInteractionID("prt-1:production", "evt-p-1")
	if err != nil {
		t.Fatalf("InboundInteractionID() = %v", err)
	}
	var stored, duplicate domain.InteractionResponse
	for i, response := range responses {
		if failures[i] != nil {
			t.Fatalf("ReceivePartnerInteraction(%d) = %v", i, failures[i])
		}
		switch response.Status {
		case domain.InteractionStored:
			stored = response
		case domain.InteractionDuplicate:
			duplicate = response
		}
	}
	if stored.InteractionID != first || duplicate.InteractionID == "" || duplicate.InteractionID == first {
		t.Fatalf("responses = %+v, want one stored under %s and one duplicate under an id of its own", responses, first)
	}

	var winner, loser servicev1.FindExternalInteractionResponse
	e.query(t, "FindExternalInteraction", &servicev1.FindExternalInteractionRequest{InteractionId: string(first)}, &winner)
	e.query(t, "FindExternalInteraction", &servicev1.FindExternalInteractionRequest{InteractionId: string(duplicate.InteractionID)}, &loser)
	key := ports.RawKeyOf("prt-1:production", first, ports.RawRequest)
	if l, w := loser.GetInteraction(), winner.GetInteraction(); l.GetDuplicateOf() != string(first) || l.GetRequestRef() != key ||
		l.GetRequestVersionId() == w.GetRequestVersionId() || raw.versions(key) != 2 {
		t.Fatalf("duplicate = %v, first = %v with %d versions; want both versions of the raw kept, one each", l, w, raw.versions(key))
	}
	normalizations := 0
	for _, row := range e.published(t) {
		if row == "gateway.normalizations com.company.gateway.external-interaction-stored.v1" {
			normalizations++
		}
	}
	if normalizations != 1 {
		t.Fatalf("outbox = %v, want the normalization of the first receipt alone", e.published(t))
	}
}
