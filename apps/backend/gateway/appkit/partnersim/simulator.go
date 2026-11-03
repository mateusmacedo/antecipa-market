package partnersim

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"sync"

	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/serviceskit"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/ports"
)

var (
	// ErrRefused is a call the partner never read: no effect, no response.
	ErrRefused = errors.New("partnersim: the partner refused the connection")

	// ErrLost is a call whose response never came back: the partner may have
	// acted on it.
	ErrLost = errors.New("partnersim: the response was lost after the call")

	ErrNoCollector = errors.New("partnersim: a command needs the collector of its message")

	// ErrNotScripted is a read the test gave no script: no answer to invent.
	ErrNotScripted = errors.New("partnersim: the read carries no script")
)

// Simulator answers every connector and operation it is asked for. The
// kernel's scripted partner keeps the effect and the observations; the
// simulator writes the raw payloads around it in its own format.
type Simulator struct {
	partner *serviceskit.Partner
	raw     ports.RawStore
	clock   port.Clock
	ids     port.IDGenerator
	table   map[int]port.ExternalClass

	mu      sync.Mutex
	scripts map[port.OutboundKey]*script
}

// script is the directive of one command and how much of it was used, in
// step with the queues of the scripted partner.
type script struct {
	directive Directive
	submitted int
	queried   int
}

type Option func(*settings)

type settings struct {
	partner []serviceskit.PartnerOption
	table   map[int]port.ExternalClass
}

// WithoutQuery is a partner with no state query.
func WithoutQuery() Option {
	return func(s *settings) { s.partner = append(s.partner, serviceskit.WithoutQuery()) }
}

func WithTable(table map[int]port.ExternalClass) Option {
	return func(s *settings) { s.table = maps.Clone(table) }
}

// New is a simulator writing its raw payloads to raw, under ids it mints from
// ids and instants read from clock.
func New(clock port.Clock, ids port.IDGenerator, raw ports.RawStore, opts ...Option) (*Simulator, error) {
	s := settings{table: DefaultTable()}
	for _, opt := range opts {
		opt(&s)
	}
	if err := ValidateTable(s.table); err != nil {
		return nil, err
	}
	return &Simulator{
		partner: serviceskit.NewPartner(clock, ids, s.partner...),
		raw:     raw, clock: clock, ids: ids, table: s.table,
		scripts: map[port.OutboundKey]*script{},
	}, nil
}

var _ ports.PartnerAdapters = (*Simulator)(nil)

// Partner is the scripted partner underneath, for what it submitted, queried
// and applied.
func (s *Simulator) Partner() *serviceskit.Partner { return s.partner }

func (s *Simulator) Table() map[int]port.ExternalClass { return maps.Clone(s.table) }

func (s *Simulator) Classify(status int) port.ExternalClass { return classify(s.table, status) }

func (s *Simulator) Command(connector domain.ConnectorKey, operation string, collector *ports.CaptureCollector) (port.ExternalCommand, error) {
	if collector == nil {
		return nil, ErrNoCollector
	}
	return command{sim: s, connector: connector, operation: operation, collector: collector}, nil
}

func (s *Simulator) Reader(_ domain.ConnectorKey, operation string) (ports.PartnerReader, error) {
	return reader{sim: s, operation: operation}, nil
}

func (s *Simulator) Normalizer(domain.ConnectorKey, string) (ports.InteractionNormalizer, error) {
	return Normalizer{}, nil
}

// Notify is a callback or a file the partner sends on its own. Its
// observation id is the scripted partner's, so a copy delivered twice reads
// as the same notice.
func (s *Simulator) Notify(notice Notice) ports.RawBody {
	b := body{Status: 200, ExternalRef: notice.ExternalRef, References: notice.References}
	if notice.Key != "" {
		observation, _ := s.partner.DuplicateNotification(notice.Key, notice.Finding, notice.ExternalRef)
		b.OutboundKey, b.Finding, b.ObservationID, b.At = string(notice.Key), findingName(notice.Finding), observation.ID, int64(observation.SourceAt)
	} else {
		b.At = int64(s.clock.Now())
	}
	return ports.RawBody{ContentType: ContentType, Body: encode(b, notice.Break)}
}

// submit scripts the partner with the directive the first time it sees the
// key and returns the reply this submission takes; the lock keeps the
// simulator's count in step with the partner's queue.
func (s *Simulator) submit(ctx context.Context, key port.OutboundKey, payload []byte) (Reply, bool, port.ExternalClass, port.Observation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sc := s.scriptOf(key, payload)
	class, observation, err := s.partner.Submit(ctx, key, payload)
	if class == 0 || sc.submitted >= len(sc.directive.Replies) {
		return Reply{}, false, class, observation, err
	}
	reply := sc.directive.Replies[sc.submitted]
	sc.submitted++
	return reply, true, class, observation, err
}

// query advances the simulator's count only when the partner took an answer
// from its queue: a partner without query, or out of answers, took none.
func (s *Simulator) query(ctx context.Context, key port.OutboundKey) (Answer, port.Observation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	observation, err := s.partner.Query(ctx, key)
	if errors.Is(err, serviceskit.ErrNoQuery) || errors.Is(err, serviceskit.ErrScriptExhausted) {
		return Answer{}, observation, err
	}
	var answer Answer
	if sc := s.scripts[key]; sc != nil && sc.queried < len(sc.directive.Queries) {
		answer = sc.directive.Queries[sc.queried]
		sc.queried++
	}
	return answer, observation, err
}

func (s *Simulator) scriptOf(key port.OutboundKey, payload []byte) *script {
	if sc, ok := s.scripts[key]; ok {
		return sc
	}
	directive, _ := scriptOf[Directive](payload)
	sc := &script{directive: directive}
	s.scripts[key] = sc
	for _, r := range directive.Replies {
		class := s.classOf(r)
		s.partner.Script(key, serviceskit.Reply{
			Class: class, Finding: findingOf(r.Finding), Applied: r.Applied || class == port.ClassAccepted, ExternalRef: r.ExternalRef, Err: faultOf(r.Fault),
		})
	}
	for _, a := range directive.Queries {
		s.partner.ScriptQuery(key, serviceskit.Answer{
			Finding: findingOf(a.Finding), FromEffect: a.FromEffect, ExternalRef: a.ExternalRef, Err: faultOf(a.Fault),
		})
	}
	return sc
}

// classOf is the class the adapter reads from the reply: no response is a
// transport outcome, a broken body is a schema mismatch, and the rest is the
// partner table.
func (s *Simulator) classOf(r Reply) port.ExternalClass {
	switch {
	case r.Fault == FaultRefused:
		return port.ClassRetryableTransport
	case r.Fault == FaultLost:
		return port.ClassUnknownSent
	case r.Break != "":
		return port.ClassSchemaMismatch
	default:
		return s.Classify(r.Status)
	}
}

func faultOf(f Fault) error {
	switch f {
	case FaultRefused:
		return ErrRefused
	case FaultLost:
		return ErrLost
	default:
		return nil
	}
}

func (s *Simulator) put(ctx context.Context, connector domain.ConnectorKey, id domain.InteractionID, part ports.RawPart, raw []byte) (domain.RawPayload, error) {
	return s.raw.Put(ctx, ports.RawKeyOf(connector, id, part), ports.RawBody{ContentType: ContentType, Body: raw})
}

// command is the ExternalCommand of one message, bound to its collector.
type command struct {
	sim       *Simulator
	connector domain.ConnectorKey
	operation string
	collector *ports.CaptureCollector
}

// Submit keeps the request before calling the partner and the response after
// (decision 17): without the request kept the partner is never called, and a
// response the store lost leaves the outcome as the partner gave it.
func (c command) Submit(ctx context.Context, key port.OutboundKey, payload []byte) (port.ExternalClass, port.Observation, error) {
	id := domain.InteractionID(c.sim.ids.NewMessageID())
	c.collector.Begin(id)
	requested, err := c.sim.put(ctx, c.connector, id, ports.RawRequest, request(c.operation, map[string]any{
		"outboundKey": string(key), "payload": asJSON(payload),
	}))
	if err != nil {
		return port.ClassRetryableTransport, port.Observation{}, fmt.Errorf("partnersim: the request was not kept, so the partner was not called: %w", err)
	}
	c.collector.Requested(requested)

	reply, answered, class, observation, err := c.sim.submit(ctx, key, payload)
	if !answered || reply.Fault != "" {
		return class, observation, err
	}
	response := encode(body{
		Status: reply.Status, OutboundKey: string(key), Finding: findingName(observation.Finding), ExternalRef: reply.ExternalRef,
		ObservationID: observation.ID, At: int64(observation.SourceAt), References: reply.References,
	}, reply.Break)
	responded, putErr := c.sim.put(ctx, c.connector, id, ports.RawResponse, response)
	if putErr != nil {
		c.collector.ResponseLost(putErr)
	} else {
		c.collector.Responded(responded, ContentType)
	}
	if _, broke := parse(response); broke != nil {
		c.collector.Broke(*broke)
	}
	return class, observation, err
}

type reader struct {
	sim       *Simulator
	operation string
}

// QueryState answers from the scripted partner, whose FromEffect answers read
// the effect the submissions left.
func (r reader) QueryState(ctx context.Context, key port.OutboundKey) (ports.RawExchange, error) {
	sent := request(r.operation, map[string]any{"query": map[string]string{"outboundKey": string(key)}})
	answer, observation, err := r.sim.query(ctx, key)
	if err != nil {
		return ports.RawExchange{}, fmt.Errorf("partnersim: query %s: %w", key, err)
	}
	received := encode(body{
		Status: 200, OutboundKey: string(key), Finding: findingName(observation.Finding), ExternalRef: observation.ExternalRef,
		ObservationID: observation.ID, At: int64(observation.SourceAt),
	}, answer.Break)
	return ports.RawExchange{
		Request:  ports.RawBody{ContentType: ContentType, Body: sent},
		Response: ports.RawBody{ContentType: ContentType, Body: received},
	}, nil
}

// Read answers with the reading its parameters script, as of the clock.
func (r reader) Read(_ context.Context, parameters []byte) (ports.RawExchange, error) {
	sent := request(r.operation, map[string]any{"read": asJSON(parameters)})
	read, ok := scriptOf[Read](parameters)
	if !ok {
		return ports.RawExchange{}, fmt.Errorf("%w: %s", ErrNotScripted, r.operation)
	}
	if err := faultOf(read.Fault); err != nil {
		return ports.RawExchange{}, fmt.Errorf("partnersim: read %s: %w", r.operation, err)
	}
	b := body{Status: 200, ExternalRef: read.ExternalRef, At: int64(r.sim.clock.Now()), References: read.References}
	if len(read.Reading) > 0 {
		b.Reading = &read.Reading
	}
	return ports.RawExchange{
		Request:  ports.RawBody{ContentType: ContentType, Body: sent},
		Response: ports.RawBody{ContentType: ContentType, Body: encode(b, read.Break)},
	}, nil
}
