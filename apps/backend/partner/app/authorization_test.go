package app

import (
	"context"
	"errors"
	"testing"

	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/execctx"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/application"
)

var everyOperation = []application.Operation{
	application.RegisterPartner{}, application.RegisterAgreement{}, application.ActivateAgreement{},
	application.RegisterIntegrationProfile{}, application.RaiseCertificateExpiryAlert{}, application.DefineOperationalPolicy{},
	application.ChangePartnerStage{}, application.DeclareCapability{}, application.RecordHomologation{},
	application.EnableCapability{}, application.SuspendCapability{}, application.ChangeRollout{},
	application.DefineProduct{}, application.RetireProduct{},
	application.FindPartner{}, application.FindPartnersByOrganization{}, application.FindAgreementsByPartner{},
	application.FindCapabilitiesByPartner{}, application.FindEffectiveCapability{}, application.FindEligiblePartners{},
	application.FindProductsByPartner{}, application.FindIntegrationProfile{}, application.FindOperationalPolicy{},
	application.FindCertificatesExpiring{},
}

func TestEveryOperationDeclaresItsPermission(t *testing.T) {
	cases := map[string]struct {
		op   application.Operation
		want port.Permission
	}{
		"RegisterPartner":   {application.RegisterPartner{}, "partner:register-partner"},
		"RegisterAgreement": {application.RegisterAgreement{}, "partner:register-agreement"},
		"ActivateAgreement": {application.ActivateAgreement{}, "partner:activate-agreement"},

		"RegisterIntegrationProfile":  {application.RegisterIntegrationProfile{}, "partner:register-integration-profile"},
		"RaiseCertificateExpiryAlert": {application.RaiseCertificateExpiryAlert{}, "partner:raise-certificate-expiry-alert"},
		"DefineOperationalPolicy":     {application.DefineOperationalPolicy{}, "partner:define-operational-policy"},

		"ChangePartnerStage": {application.ChangePartnerStage{}, "partner:change-partner-stage"},
		"DeclareCapability":  {application.DeclareCapability{}, "partner:declare-capability"},
		"RecordHomologation": {application.RecordHomologation{}, "partner:record-homologation"},
		"EnableCapability":   {application.EnableCapability{}, "partner:enable-capability"},
		"SuspendCapability":  {application.SuspendCapability{}, "partner:suspend-capability"},
		"ChangeRollout":      {application.ChangeRollout{}, "partner:change-rollout"},

		"DefineProduct": {application.DefineProduct{}, "partner:define-product"},
		"RetireProduct": {application.RetireProduct{}, "partner:retire-product"},

		"FindPartner":                {application.FindPartner{}, "partner:find-partner"},
		"FindPartnersByOrganization": {application.FindPartnersByOrganization{}, "partner:find-partners-by-organization"},
		"FindAgreementsByPartner":    {application.FindAgreementsByPartner{}, "partner:find-agreements-by-partner"},
		"FindCapabilitiesByPartner":  {application.FindCapabilitiesByPartner{}, "partner:find-capabilities-by-partner"},
		"FindEffectiveCapability":    {application.FindEffectiveCapability{}, "partner:find-effective-capability"},
		"FindEligiblePartners":       {application.FindEligiblePartners{}, "partner:find-eligible-partners"},
		"FindProductsByPartner":      {application.FindProductsByPartner{}, "partner:find-products-by-partner"},
		"FindIntegrationProfile":     {application.FindIntegrationProfile{}, "partner:find-integration-profile"},
		"FindOperationalPolicy":      {application.FindOperationalPolicy{}, "partner:find-operational-policy"},
		"FindCertificatesExpiring":   {application.FindCertificatesExpiring{}, "partner:find-certificates-expiring"},
	}
	for name, c := range cases {
		if got := permissionOf(c.op); got != c.want {
			t.Errorf("%s: permissionOf() = %q, want %q", name, got, c.want)
		}
	}
}

func TestAuthorizationDeniesASubjectWithoutThePermission(t *testing.T) {
	ctx := execctx.WithExecution(t, context.Background())

	if err := Authorization()(ctx, application.RegisterPartner{}); !errors.Is(err, port.ErrDenied) {
		t.Fatalf("Authorization() = %v, want ErrDenied", err)
	}
}

func TestAuthorizationAdmitsTheTenantScopedCallTheGRPCHopMounts(t *testing.T) {
	ctx := execctx.WithExecution(t, context.Background(), execctx.WithoutSubject(), execctx.WithTenant("acme"))

	for _, op := range everyOperation {
		if err := Authorization()(ctx, op); err != nil {
			t.Fatalf("Authorization(%T) = %v, want nil: the permission is checked at the bff", op, err)
		}
	}
}

func TestNoTwoOperationsShareAPermission(t *testing.T) {
	seen := map[port.Permission]application.Operation{}
	for _, op := range everyOperation {
		permission := permissionOf(op)
		if permission == "" {
			t.Fatalf("%T declares no permission, which Permitted denies to every caller", op)
		}
		if other, taken := seen[permission]; taken {
			t.Fatalf("%T and %T share %q", op, other, permission)
		}
		seen[permission] = op
	}
}
