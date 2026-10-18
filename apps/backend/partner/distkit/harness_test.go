//go:build integration && distributed

package distkit_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	kernelapp "github.com/mateusmacedo/dmpf/libs/backend/go/app"
	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/ids"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/dist"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/app"
	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/application"
	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/distkit"
	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/provider"
)

const records = 6

// The name is fixed: dist.Harness re-executes the binary with
// -test.run=^TestDistkitRole$ (testkit v1.0.0-rc.5 tb/dist/harness.go:29).
func TestDistkitRole(t *testing.T) { distkit.RunRole(t) }

func TestTwoDrainsPublishEveryRecordExactlyOnce(t *testing.T) {
	h := dist.New(t, distkit.Spec)
	enqueue(t, h.Pool, records)

	tb.Require(t, dist.RelayVector(t, h, records))
}

type funders struct{}

func (funders) FindOrganization(context.Context, port.TenantID, domain.OrganizationID) ([]domain.OrganizationKind, error) {
	return []domain.OrganizationKind{domain.KindFunder}, nil
}

// The message context is required: the outbox claim drains only records whose
// metadata names correlation, causation and traceparent (postgres v1.0.0-rc.5 claim.go:52-54).
func enqueue(t *testing.T, pool *pgxpool.Pool, n int) {
	t.Helper()
	service := compose(t, pool)
	ctx := port.WithMessageContext(context.Background(), port.MessageContext{
		CorrelationID: "corr-distkit",
		CausationID:   "caus-distkit",
		Traceparent:   "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01",
	})

	for i := range n {
		partner := domain.PartnerID(fmt.Sprintf("prt-distkit-%d", i))
		registered, err := service.RegisterPartner(withExecution(t, ctx), application.RegisterPartner{
			PartnerID:      partner,
			OrganizationID: domain.OrganizationID(fmt.Sprintf("org-distkit-%d", i)),
			Role:           domain.RoleFunder,
		})
		if err != nil {
			t.Fatalf("RegisterPartner(%s) = %v, want nil", partner, err)
		}
		if rejection, refused := registered.Rejection(); refused {
			t.Fatalf("RegisterPartner(%s) was rejected with %v, want accepted", partner, rejection)
		}
	}
}

func compose(t *testing.T, pool *pgxpool.Pool) application.Service {
	t.Helper()
	cfg := app.Defaults(app.RoleAPI)
	policy, err := kernelapp.IdempotencyPolicy(cfg.Policies.IdempotencyWait, cfg.Policies.IdempotencyRetention)
	if err != nil {
		t.Fatalf("IdempotencyPolicy() = %v", err)
	}
	service := app.NewService(pool, pool, funders{}, provider.RolloutBucket{}, clock.New(port.Instant(1)), &ids.Sequence{Prefix: "m-"}, app.Waits{Command: time.Second})
	service.Idempotency = policy
	return service
}
