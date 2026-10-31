package ports

import (
	"context"

	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/domain"
)

type (
	ConnectorRepository          = port.Repository[domain.ConnectorKey, domain.ConnectorSnapshot]
	ConnectorReader              = port.Reader[domain.ConnectorKey, domain.ConnectorSnapshot]
	CapabilityProfileRepository  = port.Repository[domain.ProfileKey, domain.CapabilityProfileSnapshot]
	CapabilityProfileReader      = port.Reader[domain.ProfileKey, domain.CapabilityProfileSnapshot]
	QueryAuthorizationRepository = port.Repository[domain.ConsentID, domain.QueryAuthorizationSnapshot]
	ExternalCommandRepository    = port.Repository[domain.IdempotencyKey, domain.ExternalCommandSnapshot]
	ExternalCommandReader        = port.Reader[domain.IdempotencyKey, domain.ExternalCommandSnapshot]
)

// CapabilityProfilesByConnectorReader lists every version of the profile of a
// connector: FindEffectiveCapabilityProfile reads it outside the unit of work
// and CreateExternalCommand inside it, under the guard on connector_id.
type CapabilityProfilesByConnectorReader interface {
	LoadByConnector(ctx context.Context, connector domain.ConnectorKey) ([]domain.CapabilityProfileSnapshot, error)
}

type QueryAuthorizationsByOrganizationReader interface {
	LoadByOrganization(ctx context.Context, organization domain.OrganizationID) ([]domain.QueryAuthorizationSnapshot, error)
}

// ExternalCommandsByBusinessKeyReader filters by the business key alone; the
// caller narrows to the connector, because the kernel Relation takes one column.
type ExternalCommandsByBusinessKeyReader interface {
	LoadByBusinessKey(ctx context.Context, businessKey string) ([]domain.ExternalCommandSnapshot, error)
}

type ExternalCommandsByStatusReader interface {
	LoadByStatus(ctx context.Context, status domain.CommandStatus) ([]domain.ExternalCommandSnapshot, error)
}

// ExternalCommandsBySeriesReader reads the versions of a series inside the
// unit of work of CreateExternalCommand, under the guard on series_key, so two
// versions of one series cannot both pass the check (ADR-067).
type ExternalCommandsBySeriesReader interface {
	LoadBySeries(ctx context.Context, seriesKey string) ([]domain.ExternalCommandSnapshot, error)
}

// ExternalCommandsByOutboundKeyReader finds the command of an attempt the
// kernel's sweep abandons: the ledger knows the outbound key, not the command.
type ExternalCommandsByOutboundKeyReader interface {
	LoadByOutboundKey(ctx context.Context, outboundKey port.OutboundKey) ([]domain.ExternalCommandSnapshot, error)
}
