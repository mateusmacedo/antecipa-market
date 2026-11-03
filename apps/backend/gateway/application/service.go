package application

import (
	"context"
	"log/slog"

	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/ports"
)

const (
	AggregateTypeConnector           = "gateway.Connector"
	AggregateTypeCapabilityProfile   = "gateway.CapabilityProfile"
	AggregateTypeQueryAuthorization  = "gateway.QueryAuthorization"
	AggregateTypeExternalCommand     = "gateway.ExternalCommand"
	AggregateTypeExternalInteraction = "gateway.ExternalInteraction"
	AggregateTypeExternalReference   = "gateway.ExternalReference"

	// Destination carries the integration events; the other three, each a
	// channel of one internal type (KFK-03): DispatchDestination the dispatch
	// intent, InteractionDestination the capture of a submission's response and
	// NormalizationDestination the interaction stored for normalization.
	Destination              = "gateway.events"
	DispatchDestination      = "gateway.dispatch"
	InteractionDestination   = "gateway.interactions"
	NormalizationDestination = "gateway.normalizations"

	// Each internal channel keeps a message inbox of its own, named after it
	// and apart from the one of partner.events (INB-01).
	DispatchConsumer      = DispatchDestination
	InteractionConsumer   = InteractionDestination
	NormalizationConsumer = NormalizationDestination

	OperationRegisterConnector                     = "gateway.RegisterConnector"
	OperationSuspendOperation                      = "gateway.SuspendOperation"
	OperationEnableOperation                       = "gateway.EnableOperation"
	OperationDeclareCapabilityProfile              = "gateway.DeclareCapabilityProfile"
	OperationGrantQueryAuthorization               = "gateway.GrantQueryAuthorization"
	OperationRevokeQueryAuthorization              = "gateway.RevokeQueryAuthorization"
	OperationCreateExternalCommand                 = "gateway.CreateExternalCommand"
	OperationCancelExternalCommand                 = "gateway.CancelExternalCommand"
	OperationRequestResend                         = "gateway.RequestResend"
	OperationHoldDispatch                          = "gateway.HoldDispatch"
	OperationExpireAmbiguity                       = "gateway.ExpireAmbiguity"
	OperationResolveUnknownOutcome                 = "gateway.ResolveUnknownOutcome"
	OperationSweepExternalCommands                 = "gateway.SweepExternalCommands"
	OperationConsumeCapabilityEnabled              = "gateway.ConsumeCapabilityEnabled"
	OperationConsumeCapabilitySuspended            = "gateway.ConsumeCapabilitySuspended"
	OperationFindConnector                         = "gateway.FindConnector"
	OperationFindEffectiveCapabilityProfile        = "gateway.FindEffectiveCapabilityProfile"
	OperationFindCapabilityProfile                 = "gateway.FindCapabilityProfile"
	OperationFindQueryAuthorizationByOrganization  = "gateway.FindQueryAuthorizationByOrganization"
	OperationFindExternalCommand                   = "gateway.FindExternalCommand"
	OperationFindExternalCommandsByBusinessKey     = "gateway.FindExternalCommandsByBusinessKey"
	OperationFindExternalCommandsByStatus          = "gateway.FindExternalCommandsByStatus"
	OperationRecordExternalInteraction             = "gateway.RecordExternalInteraction"
	OperationConcludeInteraction                   = "gateway.ConcludeInteraction"
	OperationReprocessInteraction                  = "gateway.ReprocessInteraction"
	OperationLinkExternalReference                 = "gateway.LinkExternalReference"
	OperationFindExternalInteraction               = "gateway.FindExternalInteraction"
	OperationFindExternalInteractionByPartnerEvent = "gateway.FindExternalInteractionByPartnerEvent"
	OperationFindExternalInteractionsByCommand     = "gateway.FindExternalInteractionsByCommand"
	OperationFindExternalReference                 = "gateway.FindExternalReference"
	OperationFindExternalReferencesBySubject       = "gateway.FindExternalReferencesBySubject"
	OperationDispatchExternalCommand               = "gateway.DispatchExternalCommand"
	OperationConsumeResponseCaptured               = "gateway.ConsumeResponseCaptured"
	OperationConsumeInteractionStored              = "gateway.ConsumeInteractionStored"

	// OperationCaptureResponse only ever fails: it is how a response the store
	// lost reaches the metrics while the submission stands (decision 17).
	OperationCaptureResponse = "gateway.CaptureResponse"
)

// A submission produces ExternalCommandSubmitted and the fact its arrival
// emits, and its capture ExternalResponseCaptured; a conclusion of a
// reprocessing, the reprocessing and the outcome it announces. Four leaves
// room for the largest of them.
const maxEventsPerCommand = 4

// captureSlot is the identifier the capture of a submission takes: the step
// emits at most the submission and the fact of its arrival, before it.
const captureSlot = maxEventsPerCommand - 1

// Resources is the set one unit of work binds: the command inbox (Commands),
// the message inbox of partner.events (Inbox), one message inbox per internal
// channel and the kernel's attempt ledger share the transaction of the state
// and the outbox.
type Resources struct {
	Connectors           ports.ConnectorRepository
	Profiles             ports.CapabilityProfileRepository
	Authorizations       ports.QueryAuthorizationRepository
	ExternalCommands     ports.ExternalCommandRepository
	ExternalInteractions ports.ExternalInteractionRepository
	ExternalReferences   ports.ExternalReferenceRepository
	ProfilesByConnector  ports.CapabilityProfilesByConnectorReader
	CommandsBySeries     ports.ExternalCommandsBySeriesReader
	Ledger               port.AttemptLedger
	Outbox               port.Outbox
	Commands             port.Inbox
	Inbox                port.Inbox
	DispatchInbox        port.Inbox
	InteractionInbox     port.Inbox
	NormalizationInbox   port.Inbox
}

type Service struct {
	UoW                          port.UnitOfWork[Resources]
	ConnectorReader              ports.ConnectorReader
	ProfileReader                ports.CapabilityProfileReader
	ProfilesByConnector          ports.CapabilityProfilesByConnectorReader
	AuthorizationsByOrganization ports.QueryAuthorizationsByOrganizationReader
	CommandReader                ports.ExternalCommandReader
	CommandsByBusinessKey        ports.ExternalCommandsByBusinessKeyReader
	CommandsByStatus             ports.ExternalCommandsByStatusReader
	CommandsByOutboundKey        ports.ExternalCommandsByOutboundKeyReader
	InteractionReader            ports.ExternalInteractionReader
	InteractionsByPartnerEvent   ports.ExternalInteractionsByPartnerEventReader
	InteractionsByCommand        ports.ExternalInteractionsByCommandReader
	ReferenceReader              ports.ExternalReferenceReader
	ReferencesBySubject          ports.ExternalReferencesBySubjectReader
	Clock                        port.Clock
	IDs                          port.IDGenerator
	Authorize                    usecase.Authorize[Operation]
	Idempotency                  usecase.IdempotencyPolicy
	OutboundKeys                 usecase.OutboundKeyPolicy
	DerivedKeys                  DerivedKeyPolicy
	Consumer                     string

	// Raw keeps the raw payloads and Partners resolves the partner adapters;
	// the runtime has none until the providers are mapped (decision 21).
	Raw      ports.RawStore
	Partners ports.PartnerAdapters

	// Attempts is the kernel's attempt ledger outside any unit of work, where
	// Dispatch begins an attempt before calling the partner (OBX-07); Budget is
	// the cut the composition root derives from app.ExternalBudget.
	Attempts port.AttemptLedger
	Budget   DispatchBudget

	Instrumentation port.Instrumentation
	Logger          *slog.Logger
}

// DispatchBudget is what the kernel's Dispatch reads of app.ExternalBudget, in
// a form this block takes without importing time: the stale cut in
// nanoseconds, shared with the sweep of attempts, and the deadline of the
// settlement.
type DispatchBudget struct {
	StaleAfter    int64
	SettleContext func(context.Context) (context.Context, context.CancelFunc)
}

type command[R any] = usecase.Command[Resources, Operation, R]

type executor = usecase.Executor[Resources, Operation]

func (s Service) executor() executor {
	return executor{
		UoW:             s.UoW,
		Inbox:           func(res Resources) port.Inbox { return res.Commands },
		Consumer:        CommandConsumer,
		Clock:           s.Clock,
		IDs:             s.IDs,
		MaxEvents:       maxEventsPerCommand,
		Policy:          s.Idempotency,
		Authorize:       s.Authorize,
		Instrumentation: s.Instrumentation,
	}
}

// executorWithin runs a command inside the unit of work a delivery already
// opened, so its command inbox entry commits with the message inbox entry.
func (s Service) executorWithin(res Resources) executor {
	x := s.executor()
	x.UoW = joined{res: res}
	return x
}

// A nil hook is the inert realization, so every operation opens and closes
// unconditionally.
func (s Service) instrumentation() port.Instrumentation {
	if s.Instrumentation == nil {
		return port.NoInstrumentation()
	}
	return s.Instrumentation
}

// A nil logger discards: the telemetry of a consumer is its own, not a
// condition of the use case.
func (s Service) logger() *slog.Logger {
	if s.Logger == nil {
		return slog.New(slog.DiscardHandler)
	}
	return s.Logger
}

// A nil adapter set is the runtime's: every way to a partner fails closed.
func (s Service) partners() ports.PartnerAdapters {
	if s.Partners == nil {
		return ports.NoPartnerAdapters{}
	}
	return s.Partners
}

func origin(aggregateType, aggregateID string) usecase.Origin {
	return usecase.Origin{Destination: Destination, AggregateType: aggregateType, AggregateID: aggregateID}
}

func dispatchOrigin(key domain.IdempotencyKey) usecase.Origin {
	return usecase.Origin{Destination: DispatchDestination, AggregateType: AggregateTypeExternalCommand, AggregateID: string(key)}
}

func normalizationOrigin(id domain.InteractionID) usecase.Origin {
	return usecase.Origin{Destination: NormalizationDestination, AggregateType: AggregateTypeExternalInteraction, AggregateID: string(id)}
}

// captureOrigin keys the capture by the interaction it names, which no write
// has created yet: version zero is the one before its record.
func captureOrigin(id domain.InteractionID) usecase.Origin {
	return usecase.Origin{Destination: InteractionDestination, AggregateType: AggregateTypeExternalInteraction, AggregateID: string(id)}
}

func ledgerOf(res Resources) port.AttemptLedger { return res.Ledger }

func dispatchInboxOf(res Resources) port.Inbox { return res.DispatchInbox }

func interactionInboxOf(res Resources) port.Inbox { return res.InteractionInbox }

func normalizationInboxOf(res Resources) port.Inbox { return res.NormalizationInbox }

var (
	loadConnector             = usecase.OrNew(domain.NewConnector, domain.FromConnectorSnapshot)
	loadExistingConnector     = usecase.Existing[domain.ConnectorKey](domain.FromConnectorSnapshot)
	loadAbsentProfile         = usecase.Absent[domain.CapabilityProfileSnapshot](domain.NewCapabilityProfile)
	loadAuthorization         = usecase.OrNew(domain.NewQueryAuthorization, domain.FromQueryAuthorizationSnapshot)
	loadExistingAuthorization = usecase.Existing[domain.ConsentID](domain.FromQueryAuthorizationSnapshot)
	loadCommand               = usecase.OrNew(domain.NewExternalCommand, domain.FromExternalCommandSnapshot)
	loadExistingCommand       = usecase.Existing[domain.IdempotencyKey](domain.FromExternalCommandSnapshot)
	loadAbsentInteraction     = usecase.Absent[domain.ExternalInteractionSnapshot](domain.NewExternalInteraction)
	loadExistingInteraction   = usecase.Existing[domain.InteractionID](domain.FromExternalInteractionSnapshot)
	loadReference             = usecase.OrNew(domain.NewExternalReference, domain.FromExternalReferenceSnapshot)
)
