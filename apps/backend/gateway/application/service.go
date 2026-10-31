package application

import (
	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/ports"
)

const (
	AggregateTypeConnector          = "gateway.Connector"
	AggregateTypeCapabilityProfile  = "gateway.CapabilityProfile"
	AggregateTypeQueryAuthorization = "gateway.QueryAuthorization"
	AggregateTypeExternalCommand    = "gateway.ExternalCommand"

	// Destination carries the integration events; DispatchDestination the
	// dispatch intent, a channel of one internal type (KFK-03).
	Destination         = "gateway.events"
	DispatchDestination = "gateway.dispatch"

	OperationRegisterConnector                    = "gateway.RegisterConnector"
	OperationSuspendOperation                     = "gateway.SuspendOperation"
	OperationEnableOperation                      = "gateway.EnableOperation"
	OperationDeclareCapabilityProfile             = "gateway.DeclareCapabilityProfile"
	OperationGrantQueryAuthorization              = "gateway.GrantQueryAuthorization"
	OperationRevokeQueryAuthorization             = "gateway.RevokeQueryAuthorization"
	OperationCreateExternalCommand                = "gateway.CreateExternalCommand"
	OperationCancelExternalCommand                = "gateway.CancelExternalCommand"
	OperationRequestResend                        = "gateway.RequestResend"
	OperationExpireAmbiguity                      = "gateway.ExpireAmbiguity"
	OperationResolveUnknownOutcome                = "gateway.ResolveUnknownOutcome"
	OperationSweepExternalCommands                = "gateway.SweepExternalCommands"
	OperationConsumeCapabilityEnabled             = "gateway.ConsumeCapabilityEnabled"
	OperationConsumeCapabilitySuspended           = "gateway.ConsumeCapabilitySuspended"
	OperationFindConnector                        = "gateway.FindConnector"
	OperationFindEffectiveCapabilityProfile       = "gateway.FindEffectiveCapabilityProfile"
	OperationFindCapabilityProfile                = "gateway.FindCapabilityProfile"
	OperationFindQueryAuthorizationByOrganization = "gateway.FindQueryAuthorizationByOrganization"
	OperationFindExternalCommand                  = "gateway.FindExternalCommand"
	OperationFindExternalCommandsByBusinessKey    = "gateway.FindExternalCommandsByBusinessKey"
	OperationFindExternalCommandsByStatus         = "gateway.FindExternalCommandsByStatus"
)

// A submission produces at most two facts: ExternalCommandSubmitted and the one
// its arrival emits.
const maxEventsPerCommand = 2

// Resources is the set one unit of work binds: the command inbox (Commands),
// the message inbox of the consumer (Inbox) and the kernel's attempt ledger
// share the transaction of the state and the outbox.
type Resources struct {
	Connectors          ports.ConnectorRepository
	Profiles            ports.CapabilityProfileRepository
	Authorizations      ports.QueryAuthorizationRepository
	ExternalCommands    ports.ExternalCommandRepository
	ProfilesByConnector ports.CapabilityProfilesByConnectorReader
	CommandsBySeries    ports.ExternalCommandsBySeriesReader
	Ledger              port.AttemptLedger
	Outbox              port.Outbox
	Commands            port.Inbox
	Inbox               port.Inbox
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
	Clock                        port.Clock
	IDs                          port.IDGenerator
	Authorize                    usecase.Authorize[Operation]
	Idempotency                  usecase.IdempotencyPolicy
	OutboundKeys                 usecase.OutboundKeyPolicy
	Consumer                     string

	Instrumentation port.Instrumentation
}

type command[R any] = usecase.Command[Resources, Operation, R]

func (s Service) executor() usecase.Executor[Resources, Operation] {
	return usecase.Executor[Resources, Operation]{
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

// A nil hook is the inert realization, so every operation opens and closes
// unconditionally.
func (s Service) instrumentation() port.Instrumentation {
	if s.Instrumentation == nil {
		return port.NoInstrumentation()
	}
	return s.Instrumentation
}

func origin(aggregateType, aggregateID string) usecase.Origin {
	return usecase.Origin{Destination: Destination, AggregateType: aggregateType, AggregateID: aggregateID}
}

func dispatchOrigin(key domain.IdempotencyKey) usecase.Origin {
	return usecase.Origin{Destination: DispatchDestination, AggregateType: AggregateTypeExternalCommand, AggregateID: string(key)}
}

func ledgerOf(res Resources) port.AttemptLedger { return res.Ledger }

var (
	loadConnector             = usecase.OrNew(domain.NewConnector, domain.FromConnectorSnapshot)
	loadExistingConnector     = usecase.Existing[domain.ConnectorKey](domain.FromConnectorSnapshot)
	loadAbsentProfile         = usecase.Absent[domain.CapabilityProfileSnapshot](domain.NewCapabilityProfile)
	loadAuthorization         = usecase.OrNew(domain.NewQueryAuthorization, domain.FromQueryAuthorizationSnapshot)
	loadExistingAuthorization = usecase.Existing[domain.ConsentID](domain.FromQueryAuthorizationSnapshot)
	loadCommand               = usecase.OrNew(domain.NewExternalCommand, domain.FromExternalCommandSnapshot)
	loadExistingCommand       = usecase.Existing[domain.IdempotencyKey](domain.FromExternalCommandSnapshot)
)
