package provider

import (
	"context"

	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/ports"
)

func NewIntegrationProfileReader(pool postgres.ReadPool) ports.IntegrationProfileReader {
	return profileTable.Reader(pool)
}

var profilesByPartner = profileTable.Relation("partner_id")

type IntegrationProfilesByPartnerReader struct{ pool postgres.ReadPool }

func NewIntegrationProfilesByPartnerReader(pool postgres.ReadPool) *IntegrationProfilesByPartnerReader {
	return &IntegrationProfilesByPartnerReader{pool: pool}
}

func (r *IntegrationProfilesByPartnerReader) LoadByPartner(ctx context.Context, partnerID domain.PartnerID) ([]domain.IntegrationProfileSnapshot, error) {
	return profilesByPartner.Query(ctx, r.pool, string(partnerID))
}

// IntegrationProfilesByPartnerWithin reads the series inside the command
// transaction, under the guard on partner_id (ADR-067).
type IntegrationProfilesByPartnerWithin struct{ tx *postgres.Tx }

func NewIntegrationProfilesByPartnerWithin(tx *postgres.Tx) *IntegrationProfilesByPartnerWithin {
	return &IntegrationProfilesByPartnerWithin{tx: tx}
}

func (r *IntegrationProfilesByPartnerWithin) LoadByPartner(ctx context.Context, partnerID domain.PartnerID) ([]domain.IntegrationProfileSnapshot, error) {
	return profilesByPartner.QueryWithin(ctx, r.tx, string(partnerID))
}

var (
	_ ports.IntegrationProfilesByPartnerReader = (*IntegrationProfilesByPartnerReader)(nil)
	_ ports.IntegrationProfilesByPartnerReader = (*IntegrationProfilesByPartnerWithin)(nil)
)

var profilesByEnvironment = profileTable.Relation("environment")

type IntegrationProfilesByEnvironmentReader struct{ pool postgres.ReadPool }

func NewIntegrationProfilesByEnvironmentReader(pool postgres.ReadPool) *IntegrationProfilesByEnvironmentReader {
	return &IntegrationProfilesByEnvironmentReader{pool: pool}
}

func (r *IntegrationProfilesByEnvironmentReader) LoadByEnvironment(ctx context.Context, environment domain.Environment) ([]domain.IntegrationProfileSnapshot, error) {
	return profilesByEnvironment.Query(ctx, r.pool, string(environment))
}

var _ ports.IntegrationProfilesByEnvironmentReader = (*IntegrationProfilesByEnvironmentReader)(nil)
