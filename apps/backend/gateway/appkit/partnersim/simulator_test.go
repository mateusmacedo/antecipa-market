package partnersim_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/memory"
	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/serviceskit"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/execctx"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/appkit/partnersim"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/ports"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/provider"
)

const (
	connector = domain.ConnectorKey("prt-1:production")
	key       = port.OutboundKey("0a1b2c")
	at        = port.Instant(1_700_000_000_000_000_000)
)

type fixture struct {
	sim *partnersim.Simulator
	raw ports.RawStore
	ctx context.Context
}

func newFixture(t *testing.T, raw func(ports.RawStore) ports.RawStore, opts ...partnersim.Option) fixture {
	t.Helper()
	store, err := memory.NewObjectStore(memory.NewObjectBucket(memory.FixedClock{At: at}),
		port.ObjectStoreDeclaration{Scope: port.ScopeTenant, Encryption: port.Encryption{Algorithm: "AES256"}})
	if err != nil {
		t.Fatalf("NewObjectStore() = %v", err)
	}
	var kept ports.RawStore = provider.NewRawStore(store, store)
	if raw != nil {
		kept = raw(kept)
	}
	sim, err := partnersim.New(memory.FixedClock{At: at}, &memory.SequenceIDs{Prefix: "sim-"}, kept, opts...)
	if err != nil {
		t.Fatalf("New() = %v", err)
	}
	ctx := execctx.WithExecution(t, context.Background(), execctx.WithSlot(), execctx.AutoKey(), execctx.WithTenant("platform"))
	return fixture{sim: sim, raw: kept, ctx: ctx}
}

type submission struct {
	class     port.ExternalClass
	finding   port.Finding
	err       error
	collector *ports.CaptureCollector
}

func (f fixture) submit(t *testing.T, payload []byte) submission {
	t.Helper()
	collector := &ports.CaptureCollector{}
	command, err := f.sim.Command(connector, "submitEffect", collector)
	if err != nil {
		t.Fatalf("Command() = %v", err)
	}
	class, observation, err := command.Submit(f.ctx, key, payload)
	return submission{class: class, finding: observation.Finding, err: err, collector: collector}
}

// read normalizes what the store kept of the side the source reads.
func (f fixture) read(t *testing.T, source domain.InteractionSource, payload domain.RawPayload) ports.Reading {
	t.Helper()
	got, err := f.raw.Get(f.ctx, payload)
	if err != nil {
		t.Fatalf("Get(%s) = %v", payload.Ref, err)
	}
	exchange := ports.RawExchange{Response: got}
	if source == domain.InteractionCallback || source == domain.InteractionFile {
		exchange = ports.RawExchange{Request: got}
	}
	return normalize(t, source, exchange)
}

func normalize(t *testing.T, source domain.InteractionSource, exchange ports.RawExchange) ports.Reading {
	t.Helper()
	reading, err := partnersim.Normalizer{}.Normalize(source, exchange)
	if err != nil {
		t.Fatalf("Normalize(%s) = %v", source, err)
	}
	again, _ := partnersim.Normalizer{}.Normalize(source, exchange)
	if again.ReadingHash != reading.ReadingHash || string(again.Canonical) != string(reading.Canonical) {
		t.Fatalf("Normalize() read one payload two ways: %q then %q", reading.Canonical, again.Canonical)
	}
	return reading
}

func (f fixture) query(t *testing.T) (ports.RawExchange, error) {
	t.Helper()
	reader, err := f.sim.Reader(connector, "submitEffect")
	if err != nil {
		t.Fatalf("Reader() = %v", err)
	}
	return reader.QueryState(f.ctx, key)
}

func captured(t *testing.T, s submission) ports.Capture {
	t.Helper()
	c, ok := s.collector.Captured()
	if !ok {
		t.Fatalf("Captured() = no capture, want both raw payloads kept (lost: %v)", s.collector.LostResponse())
	}
	return c
}

func TestASuccessKeepsBothSidesUnderTheInteractionAndReadsAsTheEffect(t *testing.T) {
	f := newFixture(t, nil)
	reference := partnersim.Reference{
		ProviderID: "prt-1", System: "registry", ReferenceType: "effect", Reference: "EF-9", SubjectType: "receivable", SubjectID: "rcv-1",
	}

	s := f.submit(t, partnersim.Payload(partnersim.Directive{Replies: []partnersim.Reply{
		{Status: 201, Finding: "applied", ExternalRef: "EF-9", References: []partnersim.Reference{reference}},
	}}))

	if s.class != port.ClassAccepted || s.finding != port.FindingApplied || s.err != nil {
		t.Fatalf("Submit() = %v %v %v, want accepted with the applied observation", s.class, s.finding, s.err)
	}
	c := captured(t, s)
	if c.InteractionID == "" || c.Request.Ref != ports.RawKeyOf(connector, c.InteractionID, ports.RawRequest) ||
		c.Response.Ref != ports.RawKeyOf(connector, c.InteractionID, ports.RawResponse) || c.ContentType != partnersim.ContentType {
		t.Fatalf("Captured() = %+v, want both sides under the interaction the adapter minted", c)
	}
	sent, err := f.raw.Get(f.ctx, c.Request)
	if err != nil || !strings.Contains(string(sent.Body), `"outboundKey":"0a1b2c"`) || !strings.Contains(string(sent.Body), `"simulator"`) {
		t.Fatalf("request = %s, %v; want the outbound key and the payload as sent", sent.Body, err)
	}
	reading := f.read(t, domain.InteractionSubmitResponse, c.Response)
	sum := sha256.Sum256(reading.Canonical)
	if reading.Outcome != domain.InteractionNormalized || reading.CommandKey != key || reading.Finding != port.FindingApplied ||
		reading.ExternalRef != "EF-9" || reading.ReadingHash != hex.EncodeToString(sum[:]) {
		t.Fatalf("reading = %+v, want the applied effect of the command under the hash of its canonical form", reading)
	}
	if len(reading.References) != 1 || reading.References[0].Reference != "EF-9" || reading.References[0].SubjectID != "rcv-1" || reading.References[0].AsOf != at {
		t.Fatalf("references = %+v, want the link the response carries as of its instant", reading.References)
	}
	if !f.sim.Partner().Applied(key) {
		t.Fatal("the partner has no effect, want the success applied")
	}
}

func TestARecoverableFailureLeavesNoEffect(t *testing.T) {
	f := newFixture(t, nil)

	upstream := f.submit(t, partnersim.Payload(partnersim.Directive{Replies: []partnersim.Reply{{Status: 503}, {Fault: partnersim.FaultRefused}}}))
	refused := f.submit(t, nil)

	if upstream.class != port.ClassRetryableUpstream || refused.class != port.ClassRetryableTransport || !errors.Is(refused.err, partnersim.ErrRefused) {
		t.Fatalf("Submit() = %v then %v (%v), want retryable upstream then transport", upstream.class, refused.class, refused.err)
	}
	if c := captured(t, upstream); c.Response.Ref == "" {
		t.Fatalf("the 503 left no response, want it kept for the outbound record: %+v", c)
	}
	if _, ok := refused.collector.Captured(); ok {
		t.Fatal("a refused call has a response, want only its request kept")
	}
	if f.sim.Partner().Applied(key) {
		t.Fatal("a retryable failure left an effect")
	}
}

func TestADefinitiveFailureReadsAsTheRefusalOfTheTable(t *testing.T) {
	for status, want := range map[int]port.ExternalClass{
		400: port.ClassFinalValidation, 403: port.ClassFinalAuthorization, 422: port.ClassFinalBusiness,
	} {
		f := newFixture(t, nil)

		s := f.submit(t, partnersim.Payload(partnersim.Directive{Replies: []partnersim.Reply{{Status: status, Finding: "rejected"}}}))

		if s.class != want {
			t.Fatalf("Submit(%d) = %v, want %v", status, s.class, want)
		}
		if reading := f.read(t, domain.InteractionSubmitResponse, captured(t, s).Response); reading.Finding != port.FindingRejected {
			t.Fatalf("reading of %d = %+v, want the rejection", status, reading)
		}
	}
}

func TestAnUnknownOutcomeIsResolvedByTheStateQuery(t *testing.T) {
	f := newFixture(t, nil)

	s := f.submit(t, partnersim.Payload(partnersim.Directive{
		Replies: []partnersim.Reply{{Fault: partnersim.FaultLost, Applied: true}},
		Queries: []partnersim.Answer{{FromEffect: true, ExternalRef: "EF-1"}},
	}))
	if s.class != port.ClassUnknownSent || !errors.Is(s.err, partnersim.ErrLost) {
		t.Fatalf("Submit() = %v %v, want unknown_sent after a lost response", s.class, s.err)
	}
	if _, ok := s.collector.Captured(); ok {
		t.Fatal("a lost response was captured")
	}

	exchange, err := f.query(t)
	if err != nil {
		t.Fatalf("QueryState() = %v", err)
	}
	reading := normalize(t, domain.InteractionStateQuery, exchange)
	if reading.Outcome != domain.InteractionNormalized || reading.CommandKey != key || reading.Finding != port.FindingApplied || reading.ExternalRef != "EF-1" {
		t.Fatalf("reading = %+v, want the effect the lost submission left", reading)
	}
	if !strings.Contains(string(exchange.Request.Body), `"query"`) {
		t.Fatalf("query request = %s, want the outbound key it asked about", exchange.Request.Body)
	}
}

func TestAnUnknownOutcomeWithoutStateQueryHasNothingToAsk(t *testing.T) {
	f := newFixture(t, nil, partnersim.WithoutQuery())

	s := f.submit(t, partnersim.Payload(partnersim.Directive{Replies: []partnersim.Reply{{Status: 504}}}))
	_, err := f.query(t)

	if s.class != port.ClassUnknownSent || !errors.Is(err, serviceskit.ErrNoQuery) {
		t.Fatalf("Submit() = %v, QueryState() = %v; want unknown_sent and no state query", s.class, err)
	}
}

// The absence confirmed by the query lets the same key go again, after the
// quiescence the kernel waits; the second submission is the one that lands.
func TestAConfirmedAbsenceLetsTheSameKeyBeSentAgain(t *testing.T) {
	f := newFixture(t, nil)
	payload := partnersim.Payload(partnersim.Directive{
		Replies: []partnersim.Reply{{Fault: partnersim.FaultLost}, {Status: 201, Finding: "applied"}},
		Queries: []partnersim.Answer{{FromEffect: true}},
	})

	first := f.submit(t, payload)
	exchange, err := f.query(t)
	if err != nil {
		t.Fatalf("QueryState() = %v", err)
	}
	absence := normalize(t, domain.InteractionStateQuery, exchange)
	resent := f.submit(t, payload)

	if first.class != port.ClassUnknownSent || absence.Finding != port.FindingAbsent || resent.class != port.ClassAccepted {
		t.Fatalf("lost %v, query %v, resend %v; want unknown_sent, absent, accepted", first.class, absence.Finding, resent.class)
	}
	if sent := f.sim.Partner().Submitted(); !slices.Equal(sent, []port.OutboundKey{key, key}) || !f.sim.Partner().Applied(key) {
		t.Fatalf("submitted %v, applied %v; want the same key twice and the effect of the second", sent, f.sim.Partner().Applied(key))
	}
}

func TestABrokenContractIsASchemaMismatchTheCaptureAndTheReadingAgreeOn(t *testing.T) {
	for _, kind := range []domain.MismatchKind{domain.MismatchStructure, domain.MismatchMissingField, domain.MismatchType, domain.MismatchVersion} {
		t.Run(string(kind), func(t *testing.T) {
			f := newFixture(t, nil)

			s := f.submit(t, partnersim.Payload(partnersim.Directive{Replies: []partnersim.Reply{{Status: 200, Break: kind}}}))

			c := captured(t, s)
			if s.class != port.ClassSchemaMismatch || c.Mismatch.Kind != kind || c.Mismatch.Detail == "" {
				t.Fatalf("Submit() = %v with %+v, want schema_mismatch of kind %s", s.class, c.Mismatch, kind)
			}
			reading := f.read(t, domain.InteractionSubmitResponse, c.Response)
			if reading.Outcome != domain.InteractionSchemaMismatch || reading.Mismatch != c.Mismatch || reading.ReadingHash != "" {
				t.Fatalf("reading = %+v, want the mismatch the capture recorded and no reading hash", reading)
			}
			if kind == domain.MismatchVersion && c.Mismatch.PartnerDeclaredVersion != "partnersim/v9" {
				t.Fatalf("mismatch = %+v, want the version the partner declared", c.Mismatch)
			}
		})
	}

	f := newFixture(t, nil)
	f.submit(t, partnersim.Payload(partnersim.Directive{
		Replies: []partnersim.Reply{{Status: 202}},
		Queries: []partnersim.Answer{{Finding: "applied", Break: domain.MismatchMissingField}},
	}))
	exchange, err := f.query(t)
	if err != nil {
		t.Fatalf("QueryState() = %v", err)
	}
	if reading := normalize(t, domain.InteractionStateQuery, exchange); reading.Outcome != domain.InteractionSchemaMismatch {
		t.Fatalf("reading of a broken query = %+v, want schema_mismatch", reading)
	}
}

func TestAnUnknownEnumerationValueIsNoBreakOfTheContract(t *testing.T) {
	raw := []byte(`{"format":"partnersim/v1","status":200,"outboundKey":"0a1b2c","finding":"settled-later"}`)

	reading := normalize(t, domain.InteractionCallback, ports.RawExchange{Request: ports.RawBody{ContentType: partnersim.ContentType, Body: raw}})

	if reading.Outcome != domain.InteractionNormalized || reading.Finding != port.FindingInconclusive {
		t.Fatalf("reading = %+v, want a normalized reading, inconclusive about the effect", reading)
	}
}

func TestANoticeNamingNoCommandIsUncorrelated(t *testing.T) {
	f := newFixture(t, nil)
	f.submit(t, partnersim.Payload(partnersim.Directive{Replies: []partnersim.Reply{{Status: 202}}}))

	correlated := normalize(t, domain.InteractionCallback, ports.RawExchange{Request: f.sim.Notify(partnersim.Notice{Key: key, Finding: port.FindingApplied})})
	stray := normalize(t, domain.InteractionFile, ports.RawExchange{Request: f.sim.Notify(partnersim.Notice{Finding: port.FindingApplied, ExternalRef: "EF-?"})})

	if correlated.Outcome != domain.InteractionNormalized || correlated.CommandKey != key || correlated.Finding != port.FindingApplied {
		t.Fatalf("callback = %+v, want the command it names", correlated)
	}
	if stray.Outcome != domain.InteractionUncorrelated || stray.ReadingHash != "" || stray.ExternalRef != "EF-?" {
		t.Fatalf("file = %+v, want uncorrelated with no reading hash", stray)
	}
}

func TestAnExternalReadAnswersWithItsReadingInCanonicalForm(t *testing.T) {
	f := newFixture(t, nil)
	reader, err := f.sim.Reader(connector, "queryAsset")
	if err != nil {
		t.Fatalf("Reader() = %v", err)
	}

	exchange, err := reader.Read(f.ctx, partnersim.Parameters(partnersim.Read{ExternalRef: "AS-1", Reading: []byte(`{"value":10.50,"asset":"AS-1"}`)}))
	if err != nil {
		t.Fatalf("Read() = %v", err)
	}
	reading := normalize(t, domain.InteractionExternalRead, exchange)

	if reading.Outcome != domain.InteractionNormalized || reading.CommandKey != "" || reading.ExternalRef != "AS-1" || reading.AsOf != at {
		t.Fatalf("reading = %+v, want the read as of the partner's clock", reading)
	}
	if !strings.Contains(string(reading.Canonical), `"reading":{"asset":"AS-1","value":10.50}`) {
		t.Fatalf("canonical = %s, want the keys sorted and the number as written", reading.Canonical)
	}
	if _, err := reader.Read(f.ctx, []byte(`{"asset":"AS-1"}`)); !errors.Is(err, partnersim.ErrNotScripted) {
		t.Fatalf("Read(unscripted) = %v, want ErrNotScripted", err)
	}
}

// raw fails the side of the interaction the test names, as a store down would.
type failingSide struct {
	ports.RawStore
	side ports.RawPart
}

func (f failingSide) Put(ctx context.Context, key string, body ports.RawBody) (domain.RawPayload, error) {
	if strings.HasSuffix(key, "/"+string(f.side)) {
		return domain.RawPayload{}, ports.ErrRawStoreUnavailable
	}
	return f.RawStore.Put(ctx, key, body)
}

func TestWithoutTheRequestKeptThePartnerIsNeverCalled(t *testing.T) {
	f := newFixture(t, func(r ports.RawStore) ports.RawStore { return failingSide{RawStore: r, side: ports.RawRequest} })

	s := f.submit(t, partnersim.Payload(partnersim.Directive{Replies: []partnersim.Reply{{Status: 201}}}))

	if s.class != port.ClassRetryableTransport || !errors.Is(s.err, ports.ErrRawStoreUnavailable) || len(f.sim.Partner().Submitted()) != 0 {
		t.Fatalf("Submit() = %v %v, submitted %v; want retryable transport with no call", s.class, s.err, f.sim.Partner().Submitted())
	}
}

func TestALostResponseLeavesTheOutcomeAsThePartnerGaveIt(t *testing.T) {
	f := newFixture(t, func(r ports.RawStore) ports.RawStore { return failingSide{RawStore: r, side: ports.RawResponse} })

	s := f.submit(t, partnersim.Payload(partnersim.Directive{Replies: []partnersim.Reply{{Status: 201, Finding: "applied"}}}))

	if _, ok := s.collector.Captured(); s.class != port.ClassAccepted || s.err != nil || ok || !errors.Is(s.collector.LostResponse(), ports.ErrRawStoreUnavailable) {
		t.Fatalf("Submit() = %v %v, captured %v, lost %v; want accepted, no capture and the failure recorded", s.class, s.err, ok, s.collector.LostResponse())
	}
}

func TestACallTheTestDidNotScriptIsAProviderDefect(t *testing.T) {
	f := newFixture(t, nil)

	s := f.submit(t, []byte(`{"effect":"EF-1"}`))

	if s.class != 0 || !errors.Is(s.err, serviceskit.ErrScriptExhausted) {
		t.Fatalf("Submit() = %v %v, want the zero class of a provider defect", s.class, s.err)
	}
	if _, err := f.sim.Command(connector, "submitEffect", nil); !errors.Is(err, partnersim.ErrNoCollector) {
		t.Fatalf("Command(nil collector) = %v, want ErrNoCollector", err)
	}
}
