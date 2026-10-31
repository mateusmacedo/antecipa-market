package ports_test

import (
	"context"
	"testing"

	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/gateway/ports"
)

type commandsByOutboundKey map[port.OutboundKey][]domain.ExternalCommandSnapshot

func (r commandsByOutboundKey) LoadByOutboundKey(_ context.Context, key port.OutboundKey) ([]domain.ExternalCommandSnapshot, error) {
	return r[key], nil
}

type commandsBySeries map[string][]domain.ExternalCommandSnapshot

func (r commandsBySeries) LoadBySeries(_ context.Context, series string) ([]domain.ExternalCommandSnapshot, error) {
	return r[series], nil
}

// The relation ports carry the domain types the use cases decide on, so a
// realization answers in snapshots, never in rows.
func TestTheRelationPortsAnswerInDomainSnapshots(t *testing.T) {
	command := domain.ExternalCommandSnapshot{Key: "idem-1", SeriesKey: "prt-1:production:submitEffect:eff-1", OutboundKey: "0a1b"}

	var byKey ports.ExternalCommandsByOutboundKeyReader = commandsByOutboundKey{"0a1b": {command}}
	var bySeries ports.ExternalCommandsBySeriesReader = commandsBySeries{command.SeriesKey: {command}}

	found, err := byKey.LoadByOutboundKey(context.Background(), "0a1b")
	if err != nil || len(found) != 1 || found[0].Key != "idem-1" {
		t.Fatalf("LoadByOutboundKey() = %+v, %v", found, err)
	}
	series, err := bySeries.LoadBySeries(context.Background(), command.SeriesKey)
	if err != nil || len(series) != 1 || !series[0].Equal(command) {
		t.Fatalf("LoadBySeries() = %+v, %v", series, err)
	}
}
