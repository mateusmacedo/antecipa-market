package application_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/execctx"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/application"
	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
)

func TestARepeatedRegisterPartnerReplaysWithoutASecondEffect(t *testing.T) {
	h, instr := newInstrumentedHarness(t)
	first, err := h.service.RegisterPartner(withKey(t, context.Background(), "k-partner"), registerPartner())
	if err != nil {
		t.Fatalf("first RegisterPartner() = %v, want nil", err)
	}
	h.rec.Reset()
	ctx := withKey(t, context.Background(), "k-partner")

	again, err := h.service.RegisterPartner(ctx, registerPartner())

	if err != nil || again.Response() != first.Response() {
		t.Fatalf("replay = %+v, %v; want %+v", again.Response(), err, first.Response())
	}
	if got := h.outboxLen(); got != 1 {
		t.Fatalf("outbox = %d entries, want 1: the replay produced a second event", got)
	}
	if len(instr.audits) != 1 {
		t.Fatalf("Audit called %d times, want 1: a replay is not a new fact", len(instr.audits))
	}
	if slices.Contains(h.rec.Observed(), "directory.FindOrganization") {
		t.Fatalf("sequence = %v: the replay reached the directory", h.rec.Observed())
	}
	if outcome, _ := port.IdempotencyOutcomeFrom(ctx); outcome != port.IdempotencyReplayed {
		t.Fatalf("idempotency outcome = %v, want replayed", outcome)
	}
}

func TestARepeatedRefusalReplaysTheRejection(t *testing.T) {
	h := newHarness(t)
	delete(h.directory.kinds, testOrganizationID)
	if _, err := h.service.RegisterPartner(withKey(t, context.Background(), "k-refused"), registerPartner()); err != nil {
		t.Fatalf("first RegisterPartner() = %v, want a rejection, not an error", err)
	}
	h.directory.kinds[testOrganizationID] = []domain.OrganizationKind{domain.KindFunder}

	again, err := h.service.RegisterPartner(withKey(t, context.Background(), "k-refused"), registerPartner())

	if err != nil {
		t.Fatalf("repeated RegisterPartner() = %v, want the stored rejection", err)
	}
	if rej, refused := again.Rejection(); !refused || rej.Code() != domain.CodePartnerOrganizationNotFound {
		t.Fatalf("replay = %+v, want the rejection %s", again, domain.CodePartnerOrganizationNotFound)
	}
}

func TestTheSameKeyWithAnotherPayloadIsAMismatch(t *testing.T) {
	cases := map[string]func(*harness) error{
		"register partner role": func(h *harness) error {
			cmd := registerPartner()
			cmd.Role = domain.RoleRegistry
			_, err := h.service.RegisterPartner(withKey(t, context.Background(), "k-reused"), cmd)
			return err
		},
		"register partner organization": func(h *harness) error {
			cmd := registerPartner()
			cmd.OrganizationID = "org-other"
			_, err := h.service.RegisterPartner(withKey(t, context.Background(), "k-reused"), cmd)
			return err
		},
	}
	for name, second := range cases {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			if _, err := h.service.RegisterPartner(withKey(t, context.Background(), "k-reused"), registerPartner()); err != nil {
				t.Fatalf("first RegisterPartner() = %v, want nil", err)
			}

			if err := second(h); !errors.Is(err, port.ErrIdempotencyMismatch) {
				t.Fatalf("second call = %v, want ErrIdempotencyMismatch", err)
			}
		})
	}
}

func TestEachFieldOfTheAgreementCommandsTakesPartInTheFingerprint(t *testing.T) {
	registers := map[string]func(*application.RegisterAgreement){
		"partner":             func(c *application.RegisterAgreement) { c.PartnerID = "prt-other" },
		"valid from":          func(c *application.RegisterAgreement) { c.ValidFrom++ },
		"valid until":         func(c *application.RegisterAgreement) { c.ValidUntil++ },
		"instrument template": func(c *application.RegisterAgreement) { c.InstrumentTemplateID = "tpl-2" },
		"supersedes":          func(c *application.RegisterAgreement) { c.SupersedesID = "agr-0" },
	}
	for name, mutate := range registers {
		t.Run("register "+name, func(t *testing.T) {
			h := newHarness(t)
			if _, err := h.service.RegisterAgreement(withKey(t, context.Background(), "k-reused"), registerAgreement()); err != nil {
				t.Fatalf("first RegisterAgreement() = %v, want nil", err)
			}
			cmd := registerAgreement()
			mutate(&cmd)

			if _, err := h.service.RegisterAgreement(withKey(t, context.Background(), "k-reused"), cmd); !errors.Is(err, port.ErrIdempotencyMismatch) {
				t.Fatalf("second RegisterAgreement() = %v, want ErrIdempotencyMismatch", err)
			}
		})
	}
	activations := map[string]func(*application.ActivateAgreement){
		"evidence":   func(c *application.ActivateAgreement) { c.ContractEvidenceID = "evd-2" },
		"criteria":   func(c *application.ActivateAgreement) { c.SatisfiedCriteria = append(c.SatisfiedCriteria, "H16") },
		"regulatory": func(c *application.ActivateAgreement) { c.RegulatoryCheckApplicable = true },
	}
	for name, mutate := range activations {
		t.Run("activate "+name, func(t *testing.T) {
			h := newHarness(t)
			seedDraft(t, h, withExecution(t, context.Background()))
			if _, err := h.service.ActivateAgreement(withKey(t, context.Background(), "k-reused"), activateAgreement()); err != nil {
				t.Fatalf("first ActivateAgreement() = %v, want nil", err)
			}
			cmd := activateAgreement()
			mutate(&cmd)

			if _, err := h.service.ActivateAgreement(withKey(t, context.Background(), "k-reused"), cmd); !errors.Is(err, port.ErrIdempotencyMismatch) {
				t.Fatalf("second ActivateAgreement() = %v, want ErrIdempotencyMismatch", err)
			}
		})
	}
}

func TestTheSameCriteriaInAnotherOrderAreTheSamePayload(t *testing.T) {
	h := newHarness(t)
	seedDraft(t, h, withExecution(t, context.Background()))
	second := activateAgreement()
	second.SatisfiedCriteria = []string{"H18", "H17", "H15", "H14", "H14"}
	if _, err := h.service.ActivateAgreement(withKey(t, context.Background(), "k-criteria"), activateAgreement()); err != nil {
		t.Fatalf("first ActivateAgreement() = %v, want nil", err)
	}

	if _, err := h.service.ActivateAgreement(withKey(t, context.Background(), "k-criteria"), second); err != nil {
		t.Fatalf("ActivateAgreement() with the criteria reordered = %v, want the replay", err)
	}
}

func TestRegisteringAnExistingPartnerUnderAnotherKeyAlreadyExists(t *testing.T) {
	h := newHarness(t)
	if _, err := h.service.RegisterPartner(withKey(t, context.Background(), "k-first"), registerPartner()); err != nil {
		t.Fatalf("first RegisterPartner() = %v, want nil", err)
	}

	_, err := h.service.RegisterPartner(withKey(t, context.Background(), "k-second"), registerPartner())

	if !errors.Is(err, port.ErrAlreadyExists) || h.outboxLen() != 1 {
		t.Fatalf("second RegisterPartner() = %v with %d outbox entries, want ErrAlreadyExists and one", err, h.outboxLen())
	}
}

func TestACommandWaitingPastTheCeilingIsInFlight(t *testing.T) {
	h := newHarness(t, withRegisterError(port.ErrRegisterTimeout))

	_, err := h.service.RegisterPartner(withExecution(t, context.Background()), registerPartner())

	if !errors.Is(err, port.ErrIdempotencyInFlight) {
		t.Fatalf("RegisterPartner() = %v, want ErrIdempotencyInFlight", err)
	}
}

func TestACommandWithoutAKeyIsRefusedBeforeAnyEffect(t *testing.T) {
	h := newHarness(t)
	ctx := port.WithExecutionContext(context.Background(), execctx.Execution(t))

	_, err := h.service.RegisterPartner(ctx, registerPartner())

	if !errors.Is(err, port.ErrIdempotencyKeyAbsent) {
		t.Fatalf("RegisterPartner() = %v, want ErrIdempotencyKeyAbsent", err)
	}
	if len(h.directory.calls) != 0 || h.outboxLen() != 0 {
		t.Fatalf("directory calls = %d, outbox = %d; want no effect", len(h.directory.calls), h.outboxLen())
	}
}

func TestEachFieldOfTheProfileAndPolicyCommandsTakesPartInTheFingerprint(t *testing.T) {
	profiles := map[string]func(*application.RegisterIntegrationProfile){
		"partner":            func(c *application.RegisterIntegrationProfile) { c.PartnerID = "prt-other" },
		"environment":        func(c *application.RegisterIntegrationProfile) { c.Environment = domain.EnvironmentHomologation },
		"change kind":        func(c *application.RegisterIntegrationProfile) { c.ChangeKind = domain.ChangeAnnounced },
		"protocols":          func(c *application.RegisterIntegrationProfile) { c.Protocols = []string{"mtls", "https"} },
		"endpoint":           func(c *application.RegisterIntegrationProfile) { c.Endpoint = "https://other.example" },
		"schema version":     func(c *application.RegisterIntegrationProfile) { c.SchemaVersion = "v4" },
		"secret reference":   func(c *application.RegisterIntegrationProfile) { c.SecretRef = "vault://other" },
		"certificate":        func(c *application.RegisterIntegrationProfile) { c.CertificateRef = "cert-other" },
		"certificate expiry": func(c *application.RegisterIntegrationProfile) { c.CertificateExpiresAt++ },
		"effective from":     func(c *application.RegisterIntegrationProfile) { c.EffectiveFrom++ },
		"supersedes":         func(c *application.RegisterIntegrationProfile) { c.SupersedesID = "ipf-0" },
	}
	for name, mutate := range profiles {
		t.Run("profile "+name, func(t *testing.T) {
			h := newHarness(t)
			if _, err := h.service.RegisterIntegrationProfile(withKey(t, context.Background(), "k-reused"), registerProfile()); err != nil {
				t.Fatalf("first RegisterIntegrationProfile() = %v, want nil", err)
			}
			cmd := registerProfile()
			mutate(&cmd)

			if _, err := h.service.RegisterIntegrationProfile(withKey(t, context.Background(), "k-reused"), cmd); !errors.Is(err, port.ErrIdempotencyMismatch) {
				t.Fatalf("second RegisterIntegrationProfile() = %v, want ErrIdempotencyMismatch", err)
			}
		})
	}
	t.Run("alert threshold", func(t *testing.T) {
		h := newHarness(t)
		seedProfile(t, h, withExecution(t, context.Background()), expiringProfile(5*day, 0))
		if _, err := h.service.RaiseCertificateExpiryAlert(withKey(t, context.Background(), "k-reused"), raiseAlert(30)); err != nil {
			t.Fatalf("first RaiseCertificateExpiryAlert() = %v, want nil", err)
		}

		if _, err := h.service.RaiseCertificateExpiryAlert(withKey(t, context.Background(), "k-reused"), raiseAlert(7)); !errors.Is(err, port.ErrIdempotencyMismatch) {
			t.Fatalf("second RaiseCertificateExpiryAlert() = %v, want ErrIdempotencyMismatch", err)
		}
	})
	policies := map[string]func(*application.DefineOperationalPolicy){
		"partner":           func(c *application.DefineOperationalPolicy) { c.PartnerID = "prt-other" },
		"effective from":    func(c *application.DefineOperationalPolicy) { c.EffectiveFrom++ },
		"supersedes":        func(c *application.DefineOperationalPolicy) { c.SupersedesID = "pol-0" },
		"rate limit":        func(c *application.DefineOperationalPolicy) { c.RateLimitPerMinute++ },
		"business days":     func(c *application.DefineOperationalPolicy) { c.BusinessDays = c.BusinessDays[:4] },
		"cutoff time":       func(c *application.DefineOperationalPolicy) { c.CutoffTime = "18:00" },
		"processing window": func(c *application.DefineOperationalPolicy) { c.ProcessingWindow = "00:00-23:59" },
		"sla":               func(c *application.DefineOperationalPolicy) { c.SlaRef = "sla-silver" },
		"fallback":          func(c *application.DefineOperationalPolicy) { c.Fallback = "" },
		"support contact":   func(c *application.DefineOperationalPolicy) { c.SupportContact = "other" },
		"incident contact":  func(c *application.DefineOperationalPolicy) { c.IncidentContact = "other" },
	}
	for name, mutate := range policies {
		t.Run("policy "+name, func(t *testing.T) {
			h := newHarness(t)
			if _, err := h.service.DefineOperationalPolicy(withKey(t, context.Background(), "k-reused"), definePolicy()); err != nil {
				t.Fatalf("first DefineOperationalPolicy() = %v, want nil", err)
			}
			cmd := definePolicy()
			mutate(&cmd)

			if _, err := h.service.DefineOperationalPolicy(withKey(t, context.Background(), "k-reused"), cmd); !errors.Is(err, port.ErrIdempotencyMismatch) {
				t.Fatalf("second DefineOperationalPolicy() = %v, want ErrIdempotencyMismatch", err)
			}
		})
	}
}

func TestTheListsOfAProfileOrPolicyDoNotShiftIntoTheNextField(t *testing.T) {
	h := newHarness(t)
	first := registerProfile()
	first.Protocols, first.Endpoint = []string{"https"}, "mtls"
	second := registerProfile()
	second.Protocols, second.Endpoint = []string{"https", "mtls"}, ""
	if _, err := h.service.RegisterIntegrationProfile(withKey(t, context.Background(), "k-shift"), first); err != nil {
		t.Fatalf("first RegisterIntegrationProfile() = %v, want nil", err)
	}

	if _, err := h.service.RegisterIntegrationProfile(withKey(t, context.Background(), "k-shift"), second); !errors.Is(err, port.ErrIdempotencyMismatch) {
		t.Fatalf("second RegisterIntegrationProfile() = %v, want ErrIdempotencyMismatch", err)
	}
}

func TestEachFieldOfTheStageAndCapabilityCommandsTakesPartInTheFingerprint(t *testing.T) {
	stages := map[string]func(*application.ChangePartnerStage){
		"target": func(c *application.ChangePartnerStage) { c.Target = domain.PartnerSuspended },
		"reason": func(c *application.ChangePartnerStage) { c.Reason = "another reason" },
	}
	for name, mutate := range stages {
		t.Run("stage "+name, func(t *testing.T) {
			h := newHarness(t)
			seedPartner(t, h, withExecution(t, context.Background()), domain.PartnerActive)
			first := changeStage(domain.PartnerOffboarding, "contract terminated")
			if _, err := h.service.ChangePartnerStage(withKey(t, context.Background(), "k-reused"), first); err != nil {
				t.Fatalf("first ChangePartnerStage() = %v, want nil", err)
			}
			cmd := first
			mutate(&cmd)

			if _, err := h.service.ChangePartnerStage(withKey(t, context.Background(), "k-reused"), cmd); !errors.Is(err, port.ErrIdempotencyMismatch) {
				t.Fatalf("second ChangePartnerStage() = %v, want ErrIdempotencyMismatch", err)
			}
		})
	}
	declarations := map[string]func(*application.DeclareCapability){
		"partner":               func(c *application.DeclareCapability) { c.PartnerID = "prt-other" },
		"operation":             func(c *application.DeclareCapability) { c.Operation = "settle" },
		"asset type":            func(c *application.DeclareCapability) { c.AssetType = "cpr" },
		"environment":           func(c *application.DeclareCapability) { c.Environment = domain.EnvironmentHomologation },
		"is write":              func(c *application.DeclareCapability) { c.IsWrite = false },
		"state query supported": func(c *application.DeclareCapability) { c.StateQuerySupported = true },
		"restriction note":      func(c *application.DeclareCapability) { c.RestrictionNote = "another note" },
		"callback applicable":   func(c *application.DeclareCapability) { c.CallbackApplicable = false },
		"supersedes":            func(c *application.DeclareCapability) { c.SupersedesID = "cap-0" },
	}
	for name, mutate := range declarations {
		t.Run("declare "+name, func(t *testing.T) {
			h := newHarness(t)
			if _, err := h.service.DeclareCapability(withKey(t, context.Background(), "k-reused"), declareCapability()); err != nil {
				t.Fatalf("first DeclareCapability() = %v, want nil", err)
			}
			cmd := declareCapability()
			mutate(&cmd)

			if _, err := h.service.DeclareCapability(withKey(t, context.Background(), "k-reused"), cmd); !errors.Is(err, port.ErrIdempotencyMismatch) {
				t.Fatalf("second DeclareCapability() = %v, want ErrIdempotencyMismatch", err)
			}
		})
	}
	capabilityCommands := map[string]struct {
		status domain.CapabilityStatus
		first  func(*harness, context.Context) error
		second func(*harness, context.Context) error
	}{
		"homologation result": {domain.CapabilityStatusDeclared,
			func(h *harness, ctx context.Context) error {
				_, err := h.service.RecordHomologation(ctx, recordHomologation(homologationOf(true, "H1")))
				return err
			},
			func(h *harness, ctx context.Context) error {
				_, err := h.service.RecordHomologation(ctx, recordHomologation(homologationOf(false, "H1")))
				return err
			}},
		"homologation evidence": {domain.CapabilityStatusDeclared,
			func(h *harness, ctx context.Context) error {
				_, err := h.service.RecordHomologation(ctx, recordHomologation(homologationOf(true, "H1")))
				return err
			},
			func(h *harness, ctx context.Context) error {
				results := homologationOf(true, "H1")
				results[0].EvidenceID = "evd-other"
				_, err := h.service.RecordHomologation(ctx, recordHomologation(results))
				return err
			}},
		"suspension reason": {domain.CapabilityStatusEnabled,
			func(h *harness, ctx context.Context) error {
				_, err := h.service.SuspendCapability(ctx, suspendCapability("partner incident"))
				return err
			},
			func(h *harness, ctx context.Context) error {
				_, err := h.service.SuspendCapability(ctx, suspendCapability("another incident"))
				return err
			}},
		"suspension trigger": {domain.CapabilityStatusEnabled,
			func(h *harness, ctx context.Context) error {
				_, err := h.service.SuspendCapability(ctx, suspendCapability("partner incident"))
				return err
			},
			func(h *harness, ctx context.Context) error {
				cmd := suspendCapability("partner incident")
				cmd.Trigger = domain.TriggerOffboarding
				_, err := h.service.SuspendCapability(ctx, cmd)
				return err
			}},
		"rollout mode": {domain.CapabilityStatusEnabled,
			func(h *harness, ctx context.Context) error {
				_, err := h.service.ChangeRollout(ctx, changeRollout())
				return err
			},
			func(h *harness, ctx context.Context) error {
				cmd := changeRollout()
				cmd.Mode = domain.RolloutEnabled
				_, err := h.service.ChangeRollout(ctx, cmd)
				return err
			}},
		"rollout percentage": {domain.CapabilityStatusEnabled,
			func(h *harness, ctx context.Context) error {
				_, err := h.service.ChangeRollout(ctx, changeRollout())
				return err
			},
			func(h *harness, ctx context.Context) error {
				cmd := changeRollout()
				cmd.Percentage = 20
				_, err := h.service.ChangeRollout(ctx, cmd)
				return err
			}},
		"rollout allowlist": {domain.CapabilityStatusEnabled,
			func(h *harness, ctx context.Context) error {
				_, err := h.service.ChangeRollout(ctx, changeRollout())
				return err
			},
			func(h *harness, ctx context.Context) error {
				cmd := changeRollout()
				cmd.AllowlistTenantIDs = []string{"tenant-a"}
				_, err := h.service.ChangeRollout(ctx, cmd)
				return err
			}},
		"rollout excluded": {domain.CapabilityStatusEnabled,
			func(h *harness, ctx context.Context) error {
				_, err := h.service.ChangeRollout(ctx, changeRollout())
				return err
			},
			func(h *harness, ctx context.Context) error {
				cmd := changeRollout()
				cmd.ExcludedTenantIDs = nil
				_, err := h.service.ChangeRollout(ctx, cmd)
				return err
			}},
	}
	for name, c := range capabilityCommands {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			seedCapability(t, h, withExecution(t, context.Background()), capabilityVersion(testCapabilityID, c.status, 1, ""))
			if err := c.first(h, withKey(t, context.Background(), "k-reused")); err != nil {
				t.Fatalf("first command = %v, want nil", err)
			}

			if err := c.second(h, withKey(t, context.Background(), "k-reused")); !errors.Is(err, port.ErrIdempotencyMismatch) {
				t.Fatalf("second command = %v, want ErrIdempotencyMismatch", err)
			}
		})
	}
}

func TestTheTenantListsOfARolloutDoNotShiftIntoEachOther(t *testing.T) {
	h := newHarness(t)
	seedCapability(t, h, withExecution(t, context.Background()), capabilityVersion(testCapabilityID, domain.CapabilityStatusEnabled, 1, ""))
	first := changeRollout()
	first.AllowlistTenantIDs, first.ExcludedTenantIDs = []string{"tenant-a"}, nil
	second := changeRollout()
	second.AllowlistTenantIDs, second.ExcludedTenantIDs = nil, []string{"tenant-a"}
	if _, err := h.service.ChangeRollout(withKey(t, context.Background(), "k-shift"), first); err != nil {
		t.Fatalf("first ChangeRollout() = %v, want nil", err)
	}

	if _, err := h.service.ChangeRollout(withKey(t, context.Background(), "k-shift"), second); !errors.Is(err, port.ErrIdempotencyMismatch) {
		t.Fatalf("second ChangeRollout() = %v, want ErrIdempotencyMismatch", err)
	}
}
