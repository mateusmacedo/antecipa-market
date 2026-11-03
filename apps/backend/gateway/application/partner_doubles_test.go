package application_test

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"testing"

	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/memory"
	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/serviceskit"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/execctx"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/appkit/partnersim"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/application"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/ports"
)

// rawStore keeps every version of a key, like the bucket of decision 16, and
// fails the writes of one side when a test asks.
type rawStore struct {
	mu          sync.Mutex
	objects     map[string][]ports.RawBody
	putFailures map[ports.RawPart]error
	getFailure  error
}

func newRawStore() *rawStore {
	return &rawStore{objects: map[string][]ports.RawBody{}, putFailures: map[ports.RawPart]error{}}
}

func (r *rawStore) failPut(part ports.RawPart, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.putFailures[part] = err
}

func (r *rawStore) failGet(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.getFailure = err
}

func (r *rawStore) Put(_ context.Context, key string, body ports.RawBody) (domain.RawPayload, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for part, err := range r.putFailures {
		if err != nil && strings.HasSuffix(key, "/"+string(part)) {
			return domain.RawPayload{}, err
		}
	}
	r.objects[key] = append(r.objects[key], body)
	digest := serviceskit.FoldDigest(body.Body)
	return domain.RawPayload{Ref: key, VersionID: "v" + strconv.Itoa(len(r.objects[key])), Hash: fmt.Sprintf("%x", digest[:])}, nil
}

func (r *rawStore) Get(_ context.Context, payload domain.RawPayload) (ports.RawBody, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.getFailure != nil {
		return ports.RawBody{}, r.getFailure
	}
	versions := r.objects[payload.Ref]
	n, err := strconv.Atoi(strings.TrimPrefix(payload.VersionID, "v"))
	if err != nil || n < 1 || n > len(versions) {
		return ports.RawBody{}, fmt.Errorf("%w: %s %s", ports.ErrRawPayloadDefect, payload.Ref, payload.VersionID)
	}
	return versions[n-1], nil
}

func (r *rawStore) versions(key string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.objects[key])
}

// adapters is the simulator with a normalizer a test may replace, counting
// every reading it makes.
type adapters struct {
	*partnersim.Simulator
	mu      sync.Mutex
	reads   int
	replace func(domain.InteractionSource, ports.RawExchange) (ports.Reading, error)
}

func (a *adapters) Normalizer(domain.ConnectorKey, string) (ports.InteractionNormalizer, error) {
	return a, nil
}

func (a *adapters) Normalize(source domain.InteractionSource, exchange ports.RawExchange) (ports.Reading, error) {
	a.mu.Lock()
	a.reads++
	replace := a.replace
	a.mu.Unlock()
	if replace != nil {
		return replace(source, exchange)
	}
	return partnersim.Normalizer{}.Normalize(source, exchange)
}

func (a *adapters) normalizeWith(replace func(domain.InteractionSource, ports.RawExchange) (ports.Reading, error)) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.replace = replace
}

func (a *adapters) readings() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.reads
}

// recorder keeps how each operation of the instrumentation ended.
type recorder struct {
	mu    sync.Mutex
	ended map[string][]port.OutcomeCategory
}

func (r *recorder) BeginOperation(ctx context.Context, operation string) (context.Context, port.EndOperation) {
	return ctx, func(result port.Result) {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.ended[operation] = append(r.ended[operation], result.Outcome)
	}
}

func (r *recorder) Audit(context.Context, port.AuditEvent) {}

func (r *recorder) outcomes(operation string) []port.OutcomeCategory {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]port.OutcomeCategory(nil), r.ended[operation]...)
}

// warnings keeps the messages logged at warning level.
type warnings struct {
	mu       sync.Mutex
	messages []string
}

func (w *warnings) Enabled(_ context.Context, level slog.Level) bool { return level >= slog.LevelWarn }

func (w *warnings) Handle(_ context.Context, record slog.Record) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.messages = append(w.messages, record.Message)
	return nil
}

func (w *warnings) WithAttrs([]slog.Attr) slog.Handler { return w }

func (w *warnings) WithGroup(string) slog.Handler { return w }

func (w *warnings) logged() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]string(nil), w.messages...)
}

// dispatching is the harness with the partner simulator behind the context's
// partner ports (04:G12), the raw store and the kernel's attempt ledger.
type dispatching struct {
	*harness
	sim       *partnersim.Simulator
	partners  *adapters
	raw       *rawStore
	telemetry *recorder
	warnings  *warnings
}

func newDispatching(t *testing.T, change func(*domain.OperationCapability), opts ...partnersim.Option) *dispatching {
	t.Helper()
	operation := simulatorOperation("submitEffect")
	if change != nil {
		change(&operation)
	}
	h := ready(t, operation)
	raw := newRawStore()
	sim, err := partnersim.New(h.clock, &memory.SequenceIDs{Prefix: "int-"}, raw, opts...)
	if err != nil {
		t.Fatalf("partnersim.New() = %v", err)
	}
	d := &dispatching{
		harness: h, sim: sim, partners: &adapters{Simulator: sim}, raw: raw,
		telemetry: &recorder{ended: map[string][]port.OutcomeCategory{}}, warnings: &warnings{},
	}
	h.service.Raw, h.service.Partners = raw, d.partners
	h.service.Attempts = memory.NewAttemptStore(h.store)
	h.service.Budget = application.DispatchBudget{
		StaleAfter:    int64(staleAfter),
		SettleContext: func(ctx context.Context) (context.Context, context.CancelFunc) { return context.WithCancel(ctx) },
	}
	h.service.Instrumentation = d.telemetry
	h.service.Logger = slog.New(d.warnings)
	return d
}

// scripted creates the command whose payload carries the simulator's script.
func (d *dispatching) scripted(t *testing.T, directive partnersim.Directive) domain.ExternalCommandSnapshot {
	t.Helper()
	cmd := createCommand(testCommand, 1)
	cmd.Payload = string(partnersim.Payload(directive))
	must(d.service.CreateExternalCommand(withExecution(t), cmd)).accepted(t)
	return d.command(t, testCommand)
}

// consuming is the execution a consumer mounts: the tenant, no command key.
func consuming(t *testing.T) context.Context {
	t.Helper()
	return execctx.WithExecution(t, context.Background(), execctx.WithTenant(testTenant))
}

func delivery(message port.MessageID, messageType string, at port.Instant) port.Receipt {
	return port.Receipt{MessageID: message, MessageType: messageType, PayloadHash: "h-" + string(message), ReceivedAt: at}
}

func (d *dispatching) dispatch(t *testing.T, message port.MessageID) (usecase.Disposition, error) {
	t.Helper()
	return d.service.DispatchExternalCommand(consuming(t), application.DispatchExternalCommand{
		Receipt:        delivery(message, "com.company.gateway.external-command-dispatch-requested.v1", d.clock.Now()),
		IdempotencyKey: testCommand,
	})
}

func (d *dispatching) dispatched(t *testing.T, message port.MessageID, want usecase.Disposition) {
	t.Helper()
	if disposition, err := d.dispatch(t, message); err != nil || disposition != want {
		t.Fatalf("DispatchExternalCommand(%s) = %v, %v, want %v", message, disposition, err, want)
	}
}

func (d *dispatching) captures() []domain.ExternalResponseCaptured {
	var out []domain.ExternalResponseCaptured
	for _, e := range d.eventsTo(application.InteractionDestination) {
		out = append(out, e.(domain.ExternalResponseCaptured))
	}
	return out
}

func (d *dispatching) handovers() []domain.ExternalInteractionStored {
	var out []domain.ExternalInteractionStored
	for _, e := range d.eventsTo(application.NormalizationDestination) {
		out = append(out, e.(domain.ExternalInteractionStored))
	}
	return out
}

func (d *dispatching) recordCapture(t *testing.T, message port.MessageID, capture domain.ExternalResponseCaptured) usecase.Disposition {
	t.Helper()
	disposition, err := d.service.ConsumeResponseCaptured(consuming(t), application.ConsumeResponseCaptured{
		Receipt: delivery(message, "com.company.gateway.external-response-captured.v1", d.clock.Now()), Capture: capture,
	})
	if err != nil {
		t.Fatalf("ConsumeResponseCaptured(%s) = %v, %v", message, disposition, err)
	}
	return disposition
}

func (d *dispatching) normalize(t *testing.T, message port.MessageID, stored domain.ExternalInteractionStored) (usecase.Disposition, error) {
	t.Helper()
	return d.service.ConsumeInteractionStored(consuming(t), application.ConsumeInteractionStored{
		Receipt:       delivery(message, "com.company.gateway.external-interaction-stored.v1", d.clock.Now()),
		InteractionID: stored.InteractionID, Revision: stored.NormalizationRevision,
	})
}

func (d *dispatching) normalized(t *testing.T, message port.MessageID, stored domain.ExternalInteractionStored, want usecase.Disposition) {
	t.Helper()
	if disposition, err := d.normalize(t, message, stored); err != nil || disposition != want {
		t.Fatalf("ConsumeInteractionStored(%s) = %v, %v, want %v", message, disposition, err, want)
	}
}

// observed tells whether the kernel's ledger already keeps an observation id:
// a repeated id comes back as a duplicate.
func (d *dispatching) observed(t *testing.T, key port.OutboundKey, finding port.Finding, id string) bool {
	t.Helper()
	got, err := memory.NewAttemptStore(d.store).Observe(consuming(t), port.Observation{Key: key, Finding: finding, ID: id}, d.clock.Now())
	if err != nil {
		t.Fatalf("Observe(%s) = %v", id, err)
	}
	return got.Result == port.ObserveDuplicate
}
