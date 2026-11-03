package application_test

import (
	"errors"
	"math"
	"strings"
	"testing"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/serviceskit"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/application"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/ports"
)

// usable is an interaction id every place that carries it accepts: the raw key
// in the store and the record key the command inbox derives from it.
func usable(t *testing.T, policy application.DerivedKeyPolicy, id domain.InteractionID) {
	t.Helper()
	if !port.ValidIdempotencyKey(string(id)) {
		t.Fatalf("id %q is outside the idempotency key alphabet or past 128", id)
	}
	if err := port.ObjectKey(ports.RawKeyOf(testConnector, id, ports.RawRequest)).Validate(); err != nil {
		t.Fatalf("raw key of %q: %v", id, err)
	}
	if record, err := policy.Key("record", string(id)); err != nil || !port.ValidIdempotencyKey(record) {
		t.Fatalf("record key of %q = %q, %v", id, record, err)
	}
}

func TestTheFirstReceiptOfAPartnerEventDerivesItsInteractionIdFromTheConnectorAndTheEvent(t *testing.T) {
	policy := application.DerivedKeyPolicy{Digest: serviceskit.FoldDigest}

	first, err := policy.InboundInteractionID(testConnector, "evt/../p:1 ç")
	if err != nil || !strings.HasPrefix(string(first), "inbound.") {
		t.Fatalf("InboundInteractionID() = %q, %v; want a digest under inbound.", first, err)
	}
	usable(t, policy, first)
	if again, _ := policy.InboundInteractionID(testConnector, "evt/../p:1 ç"); again != first {
		t.Fatalf("InboundInteractionID() = %q then %q, want one id per event", first, again)
	}
	if other, _ := policy.InboundInteractionID("prt-2:production", "evt/../p:1 ç"); other == first {
		t.Fatalf("the same event of two connectors derived %q twice, want one id per connector", first)
	}
	if _, err := (application.DerivedKeyPolicy{}).InboundInteractionID(testConnector, "evt-1"); !errors.Is(err, application.ErrNoKeyDigest) {
		t.Fatalf("InboundInteractionID() without digest = %v, want ErrNoKeyDigest", err)
	}
}

func TestTheStateQueryDerivesItsInteractionIdFromTheCommandAndItsDueInstant(t *testing.T) {
	policy := application.DerivedKeyPolicy{Digest: serviceskit.FoldDigest}

	readable, err := policy.QueryInteractionID("idem-1", 1_700_000_000_000_000_000)
	if err != nil || readable != "query.idem-1.1700000000000000000" {
		t.Fatalf("QueryInteractionID() = %q, %v; want the readable id", readable, err)
	}
	if next, _ := policy.QueryInteractionID("idem-1", 1_700_000_000_000_000_001); next == readable {
		t.Fatal("two due instants derived one id, want one query per instant")
	}

	// The worst case: a command key at the 128 of IDM-02 and the widest instant.
	worst, err := policy.QueryInteractionID(domain.IdempotencyKey(strings.Repeat("k", 128)), kernel.Instant(math.MaxInt64))
	if err != nil {
		t.Fatalf("QueryInteractionID(worst) = %v", err)
	}
	usable(t, policy, worst)
	if again, _ := policy.QueryInteractionID(domain.IdempotencyKey(strings.Repeat("k", 128)), kernel.Instant(math.MaxInt64)); again != worst {
		t.Fatalf("QueryInteractionID(worst) = %q then %q, want a repeated sweep to derive the same id", worst, again)
	}
}
