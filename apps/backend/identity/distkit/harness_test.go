//go:build integration && distributed

package distkit_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/ids"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/dist"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/app"
	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/application"
	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/distkit"
	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/provider"
)

const establishments = 6

const records = establishments + 1

const identifierKey = "0123456789abcdef0123456789abcdef"

// The name is fixed: dist.Harness re-executes the binary with
// -test.run=^TestDistkitRole$ (testkit v1.0.0-rc.4 tb/dist/harness.go:29,142).
func TestDistkitRole(t *testing.T) { distkit.RunRole(t) }

func TestTwoDrainsPublishEveryRecordExactlyOnce(t *testing.T) {
	h := dist.New(t, distkit.Spec)
	enqueue(t, h.Pool, establishments)

	tb.Require(t, dist.RelayVector(t, h, records))
}

// The message context is required: the OBX-08 claim drains only records whose
// metadata names correlation, causation and traceparent.
func enqueue(t *testing.T, pool *pgxpool.Pool, n int) {
	t.Helper()
	service := compose(t, pool)
	ctx := port.WithMessageContext(context.Background(), port.MessageContext{
		CorrelationID: "corr-distkit",
		CausationID:   "caus-distkit",
		Traceparent:   "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01",
	})

	organization := domain.OrganizationID("org-distkit")
	registered, err := service.RegisterOrganization(withExecution(t, ctx), application.RegisterOrganization{
		OrganizationID: organization,
		Kinds:          []domain.OrganizationKind{domain.KindClient},
		Jurisdiction:   "BR",
		TaxIDType:      domain.TaxIDTypeCNPJ,
		TaxID:          "12ABC34501DE35",
		LegalName:      "Recebiveis Sinteticos Ltda",
	})
	if err != nil {
		t.Fatalf("RegisterOrganization() = %v, want nil", err)
	}
	if rejection, refused := registered.Rejection(); refused {
		t.Fatalf("RegisterOrganization() was rejected with %v, want accepted", rejection)
	}

	for i := range n {
		establishment := domain.EstablishmentID(fmt.Sprintf("est-distkit-%d", i))
		added, err := service.AddEstablishment(withExecution(t, ctx), application.AddEstablishment{
			EstablishmentID: establishment,
			OrganizationID:  organization,
			TaxIDType:       domain.TaxIDTypeCNPJ,
			TaxID:           fmt.Sprintf("12ABC345%04d35", i+2),
			TradeName:       "Loja Sintetica",
			Receiver:        true,
		})
		if err != nil {
			t.Fatalf("AddEstablishment(%s) = %v, want nil", establishment, err)
		}
		if rejection, refused := added.Rejection(); refused {
			t.Fatalf("AddEstablishment(%s) was rejected with %v, want accepted", establishment, rejection)
		}
	}
}

func compose(t *testing.T, pool *pgxpool.Pool) application.Service {
	t.Helper()
	cfg := app.Defaults(app.RoleAPI)
	cfg.IdentifierKey, cfg.IdentifierKeyVersion = app.NewSecret(identifierKey), 1
	policy, err := app.IdempotencyPolicy(cfg)
	if err != nil {
		t.Fatalf("IdempotencyPolicy() = %v", err)
	}
	protector, err := provider.NewIdentifierProtector([]byte(identifierKey), cfg.IdentifierKeyVersion)
	if err != nil {
		t.Fatalf("NewIdentifierProtector() = %v", err)
	}
	service := app.NewService(pool, pool, protector, clock.New(port.Instant(1)), &ids.Sequence{Prefix: "m-"}, app.Waits{Command: time.Second})
	service.Idempotency = policy
	return service
}
