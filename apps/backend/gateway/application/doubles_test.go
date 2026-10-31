package application_test

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"

	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/memory"
	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/serviceskit"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/execctx"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/application"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/domain"
)

const (
	testConnector = domain.ConnectorKey("prt-1:production")
	testCommand   = domain.IdempotencyKey("idem-1")
	testTenant    = port.TenantID("platform")
	second        = port.Instant(1_000_000_000)
)

var testOccurred = port.Instant(1_755_432_000_000_000_000)

var (
	connectorsTable = memory.Table[domain.ConnectorKey, domain.ConnectorSnapshot]{
		Name: "connectors", Clone: func(s domain.ConnectorSnapshot) domain.ConnectorSnapshot {
			return domain.FromConnectorSnapshot(s).Snapshot()
		},
	}
	profilesTable = memory.Table[domain.ProfileKey, domain.CapabilityProfileSnapshot]{
		Name: "capability_profiles", Clone: func(s domain.CapabilityProfileSnapshot) domain.CapabilityProfileSnapshot {
			return domain.FromCapabilityProfileSnapshot(s).Snapshot()
		},
	}
	authorizationsTable = memory.Table[domain.ConsentID, domain.QueryAuthorizationSnapshot]{
		Name: "query_authorizations", Clone: func(s domain.QueryAuthorizationSnapshot) domain.QueryAuthorizationSnapshot {
			return domain.FromQueryAuthorizationSnapshot(s).Snapshot()
		},
	}
	commandsTable = memory.Table[domain.IdempotencyKey, domain.ExternalCommandSnapshot]{Name: "external_commands"}
)

// identities records every id ever saved to a table, so a relation reader can
// scan it through the store: the memory store loads by id only.
type identities[ID comparable] struct {
	mu   sync.Mutex
	seen []ID
}

func (x *identities[ID]) add(id ID) {
	x.mu.Lock()
	defer x.mu.Unlock()
	if !slices.Contains(x.seen, id) {
		x.seen = append(x.seen, id)
	}
}

func (x *identities[ID]) all() []ID {
	x.mu.Lock()
	defer x.mu.Unlock()
	return slices.Clone(x.seen)
}

type indexed[ID comparable, S any] struct {
	port.Repository[ID, S]
	ids *identities[ID]
}

func (r indexed[ID, S]) Save(ctx context.Context, id ID, state S, expected port.Version) error {
	if err := r.Repository.Save(ctx, id, state, expected); err != nil {
		return err
	}
	r.ids.add(id)
	return nil
}

func scan[ID comparable, S any](ctx context.Context, ids *identities[ID], load func(context.Context, ID) (S, port.Version, error), keep func(S) bool) ([]S, error) {
	var out []S
	for _, id := range ids.all() {
		s, _, err := load(ctx, id)
		switch {
		case errors.Is(err, port.ErrNotFound):
			continue
		case err != nil:
			return nil, err
		}
		if keep(s) {
			out = append(out, s)
		}
	}
	return out, nil
}

type profilesByConnector struct {
	ids  *identities[domain.ProfileKey]
	load func(context.Context, domain.ProfileKey) (domain.CapabilityProfileSnapshot, port.Version, error)
}

func (r profilesByConnector) LoadByConnector(ctx context.Context, connector domain.ConnectorKey) ([]domain.CapabilityProfileSnapshot, error) {
	return scan(ctx, r.ids, r.load, func(s domain.CapabilityProfileSnapshot) bool { return s.ConnectorKey == connector })
}

type authorizationsByOrganization struct {
	ids  *identities[domain.ConsentID]
	load func(context.Context, domain.ConsentID) (domain.QueryAuthorizationSnapshot, port.Version, error)
}

func (r authorizationsByOrganization) LoadByOrganization(ctx context.Context, organization domain.OrganizationID) ([]domain.QueryAuthorizationSnapshot, error) {
	return scan(ctx, r.ids, r.load, func(s domain.QueryAuthorizationSnapshot) bool { return s.OrganizationID == organization })
}

type commandsBy struct {
	ids  *identities[domain.IdempotencyKey]
	load func(context.Context, domain.IdempotencyKey) (domain.ExternalCommandSnapshot, port.Version, error)
}

func (r commandsBy) LoadBySeries(ctx context.Context, series string) ([]domain.ExternalCommandSnapshot, error) {
	return scan(ctx, r.ids, r.load, func(s domain.ExternalCommandSnapshot) bool { return s.SeriesKey == series })
}

func (r commandsBy) LoadByBusinessKey(ctx context.Context, businessKey string) ([]domain.ExternalCommandSnapshot, error) {
	return scan(ctx, r.ids, r.load, func(s domain.ExternalCommandSnapshot) bool { return s.BusinessKey == businessKey })
}

func (r commandsBy) LoadByStatus(ctx context.Context, status domain.CommandStatus) ([]domain.ExternalCommandSnapshot, error) {
	return scan(ctx, r.ids, r.load, func(s domain.ExternalCommandSnapshot) bool { return s.Status == status })
}

func (r commandsBy) LoadByOutboundKey(ctx context.Context, key port.OutboundKey) ([]domain.ExternalCommandSnapshot, error) {
	return scan(ctx, r.ids, r.load, func(s domain.ExternalCommandSnapshot) bool { return s.OutboundKey == string(key) })
}

// clock is settable so a test can let a quiescence or a deadline elapse.
type clock struct {
	mu sync.Mutex
	at port.Instant
}

func (c *clock) Now() port.Instant {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.at
}

func (c *clock) advance(by port.Instant) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.at += by
}

type harness struct {
	fakes    *serviceskit.Fakes
	store    *memory.Store
	clock    *clock
	service  application.Service
	profiles *identities[domain.ProfileKey]
	commands *identities[domain.IdempotencyKey]
}

type option func(*setup)

type setup struct {
	faults serviceskit.Faults
}

func withSaveError(err error) option { return func(s *setup) { s.faults.Save = err } }

func withEnqueueError(err error) option { return func(s *setup) { s.faults.Enqueue = err } }

func newHarness(t *testing.T, options ...option) *harness {
	t.Helper()
	cfg := &setup{}
	for _, apply := range options {
		apply(cfg)
	}
	fakes := serviceskit.NewFakes()
	h := &harness{
		fakes: fakes, store: fakes.Store, clock: &clock{at: testOccurred},
		profiles: &identities[domain.ProfileKey]{}, commands: &identities[domain.IdempotencyKey]{},
	}
	authorizations := &identities[domain.ConsentID]{}

	bind := func(tx serviceskit.Tx) application.Resources {
		commands := indexed[domain.IdempotencyKey, domain.ExternalCommandSnapshot]{
			Repository: serviceskit.FaultyRepository(commandsTable.Repository(tx.Memory()), cfg.faults), ids: h.commands,
		}
		profiles := indexed[domain.ProfileKey, domain.CapabilityProfileSnapshot]{Repository: profilesTable.Repository(tx.Memory()), ids: h.profiles}
		return application.Resources{
			Connectors:          connectorsTable.Repository(tx.Memory()),
			Profiles:            profiles,
			Authorizations:      indexed[domain.ConsentID, domain.QueryAuthorizationSnapshot]{Repository: authorizationsTable.Repository(tx.Memory()), ids: authorizations},
			ExternalCommands:    commands,
			ProfilesByConnector: profilesByConnector{ids: h.profiles, load: profiles.Load},
			CommandsBySeries:    commandsBy{ids: h.commands, load: commands.Load},
			Ledger:              tx.Memory().AttemptLedger(),
			Outbox:              serviceskit.FaultyOutbox(tx.Outbox(), cfg.faults),
			Commands:            tx.CommandInbox(application.CommandConsumer),
			Inbox:               tx.Inbox("gateway"),
		}
	}
	commandReader := commandsTable.Reader(h.store)
	h.service = application.Service{
		UoW:                          serviceskit.UnitOfWork(fakes, bind),
		ConnectorReader:              connectorsTable.Reader(h.store),
		ProfileReader:                profilesTable.Reader(h.store),
		ProfilesByConnector:          profilesByConnector{ids: h.profiles, load: profilesTable.Reader(h.store).Load},
		AuthorizationsByOrganization: authorizationsByOrganization{ids: authorizations, load: authorizationsTable.Reader(h.store).Load},
		CommandReader:                commandReader,
		CommandsByBusinessKey:        commandsBy{ids: h.commands, load: commandReader.Load},
		CommandsByStatus:             commandsBy{ids: h.commands, load: commandReader.Load},
		CommandsByOutboundKey:        commandsBy{ids: h.commands, load: commandReader.Load},
		Clock:                        h.clock,
		IDs:                          &memory.SequenceIDs{Prefix: "m-"},
		Authorize:                    usecase.AllowAll[application.Operation](),
		Idempotency:                  usecase.IdempotencyPolicy{Wait: 1_000_000_000, Retention: 86_400_000_000_000, Digest: serviceskit.FoldDigest},
		OutboundKeys:                 usecase.OutboundKeyPolicy{Digest: serviceskit.FoldDigest},
		Consumer:                     "gateway",
	}
	return h
}

func withExecution(t *testing.T) context.Context {
	t.Helper()
	return execctx.WithExecution(t, context.Background(), execctx.WithSlot(), execctx.AutoKey(), execctx.WithTenant(testTenant))
}

func withKey(t *testing.T, key string) context.Context {
	t.Helper()
	return execctx.WithKey(t, context.Background(), key, execctx.WithSlot(), execctx.WithTenant(testTenant))
}

type result[R any] struct {
	outcome usecase.Outcome[R]
	err     error
}

func must[R any](outcome usecase.Outcome[R], err error) result[R] {
	return result[R]{outcome: outcome, err: err}
}

func (r result[R]) accepted(t *testing.T) R {
	t.Helper()
	if r.err != nil {
		t.Fatalf("use case failed: %v", r.err)
	}
	if rejection, refused := r.outcome.Rejection(); refused {
		t.Fatalf("use case refused: %v", rejection)
	}
	return r.outcome.Response()
}

func (r result[R]) refused(t *testing.T, code string) {
	t.Helper()
	if r.err != nil {
		t.Fatalf("use case failed: %v", r.err)
	}
	rejection, refused := r.outcome.Rejection()
	if !refused || string(rejection.Code()) != code {
		t.Fatalf("outcome = %+v, want the refusal %s", r.outcome, code)
	}
}

// entries returns the outbox entries the store committed, in order.
func (h *harness) entries() []port.OutboxEntry { return h.store.Entries() }

func (h *harness) command(t *testing.T, key domain.IdempotencyKey) domain.ExternalCommandSnapshot {
	t.Helper()
	s, _, err := commandsTable.Reader(h.store).Load(withExecution(t), key)
	if err != nil {
		t.Fatalf("load command %s: %v", key, err)
	}
	return s
}

func (h *harness) connector(t *testing.T) domain.ConnectorSnapshot {
	t.Helper()
	s, _, err := connectorsTable.Reader(h.store).Load(withExecution(t), testConnector)
	if err != nil {
		t.Fatalf("load connector: %v", err)
	}
	return s
}
