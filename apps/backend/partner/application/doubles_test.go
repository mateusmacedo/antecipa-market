package application_test

import (
	"context"
	"strings"
	"testing"

	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/memory"
	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/serviceskit"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/application"
	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
)

const (
	testPartnerID      = domain.PartnerID("prt-100")
	testOrganizationID = domain.OrganizationID("org-100")
	testAgreementID    = domain.AgreementID("agr-100")
	testProfileID      = domain.ProfileID("ipf-100")
	testPolicyID       = domain.PolicyID("pol-100")
	testCapabilityID   = domain.CapabilityID("cap-100")
	testProductID      = domain.ProductID("fpr-100")
	testTenant         = port.TenantID("acme")
	day                = kernel.Instant(86_400_000_000_000)
)

var testOccurred = port.Instant(1_755_432_000_000_000_000)

var (
	partnerTable = memory.Table[domain.PartnerID, domain.PartnerSnapshot]{Name: "partners", Unique: []memory.Unique[domain.PartnerSnapshot]{{
		Name: "partners_tenant_id_organization_id_role_key",
		Key: func(s domain.PartnerSnapshot) ([]string, bool) {
			return []string{string(s.OrganizationID), string(s.Role)}, true
		},
		Rejection: domain.RejectPartnerAlreadyRegistered,
	}}}
	agreementTable = memory.Table[domain.AgreementID, domain.PartnerAgreementSnapshot]{Name: "partner_agreements", Unique: seriesUniques("partner_agreements",
		func(s domain.PartnerAgreementSnapshot) (string, string) {
			return string(s.SupersedesID), string(s.PartnerID)
		},
		domain.RejectPartnerAgreementSupersedesInvalid)}
	profileTable = memory.Table[domain.ProfileID, domain.IntegrationProfileSnapshot]{Name: "integration_profiles", Unique: seriesUniques("integration_profiles",
		func(s domain.IntegrationProfileSnapshot) (string, string) {
			return string(s.SupersedesID), seriesKey(string(s.PartnerID), string(s.Environment))
		},
		domain.RejectIntegrationProfileSupersedesInvalid)}
	policyTable = memory.Table[domain.PolicyID, domain.OperationalPolicySnapshot]{Name: "operational_policies", Unique: seriesUniques("operational_policies",
		func(s domain.OperationalPolicySnapshot) (string, string) {
			return string(s.SupersedesID), string(s.PartnerID)
		},
		domain.RejectOperationalPolicySupersedesInvalid)}
	capTable = memory.Table[domain.CapabilityID, domain.CapabilitySnapshot]{Name: "capabilities", Unique: seriesUniques("capabilities",
		func(s domain.CapabilitySnapshot) (string, string) {
			return string(s.SupersedesID), seriesKey(string(s.PartnerID), s.Operation, s.AssetType, string(s.Environment))
		},
		domain.RejectCapabilitySupersedesInvalid)}
	productTable = memory.Table[domain.ProductID, domain.FinancialProductSnapshot]{Name: "financial_products", Unique: seriesUniques("financial_products",
		func(s domain.FinancialProductSnapshot) (string, string) {
			return string(s.SupersedesID), seriesKey(string(s.PartnerID), s.Code)
		},
		domain.RejectFinancialProductSupersedesInvalid)}
)

func seriesKey(parts ...string) string { return strings.Join(parts, "\x1f") }

// seriesUniques mirrors the two constraints of a versioned table: supersedes
// names one version once, and a series holds one first version, whose
// series_origin is the series key; a later version has none.
func seriesUniques[S any](table string, keys func(S) (supersedes, series string), rejection *kernel.Rejection) []memory.Unique[S] {
	return []memory.Unique[S]{
		{
			Name: table + "_tenant_id_supersedes_id_key",
			Key: func(s S) ([]string, bool) {
				supersedes, _ := keys(s)
				return []string{supersedes}, supersedes != ""
			},
			Rejection: rejection,
		},
		{
			Name: table + "_tenant_id_series_origin_key",
			Key: func(s S) ([]string, bool) {
				supersedes, series := keys(s)
				return []string{series}, supersedes == ""
			},
			Rejection: rejection,
		},
	}
}

type harness struct {
	fakes       *serviceskit.Fakes
	store       *memory.Store
	rec         *serviceskit.Steps
	directory   *fakeDirectory
	partners    *stubRelation[domain.OrganizationID, domain.PartnerSnapshot]
	agreements  *stubRelation[domain.PartnerID, domain.PartnerAgreementSnapshot]
	profiles    *stubRelation[domain.PartnerID, domain.IntegrationProfileSnapshot]
	policies    *stubRelation[domain.PartnerID, domain.OperationalPolicySnapshot]
	caps        *stubRelation[domain.PartnerID, domain.CapabilitySnapshot]
	products    *stubRelation[domain.PartnerID, domain.FinancialProductSnapshot]
	byRole      *stubRelation[domain.PartnerRole, domain.PartnerSnapshot]
	byEnv       *stubRelation[domain.Environment, domain.IntegrationProfileSnapshot]
	partnerLoad *recordingReader[domain.PartnerID, domain.PartnerSnapshot]
	service     application.Service
	partnerRead port.Reader[domain.PartnerID, domain.PartnerSnapshot]
	agreeRead   port.Reader[domain.AgreementID, domain.PartnerAgreementSnapshot]
	profileRead port.Reader[domain.ProfileID, domain.IntegrationProfileSnapshot]
	policyRead  port.Reader[domain.PolicyID, domain.OperationalPolicySnapshot]
	capRead     port.Reader[domain.CapabilityID, domain.CapabilitySnapshot]
	productRead port.Reader[domain.ProductID, domain.FinancialProductSnapshot]
}

type recordingRepo[ID comparable, S any] struct {
	table string
	inner port.Repository[ID, S]
	rec   *serviceskit.Steps
}

func (r recordingRepo[ID, S]) Load(ctx context.Context, id ID) (S, port.Version, error) {
	r.rec.Record(r.table + ".Load")
	return r.inner.Load(ctx, id)
}

func (r recordingRepo[ID, S]) Save(ctx context.Context, id ID, s S, expected port.Version) error {
	r.rec.Record(r.table + ".Save")
	return r.inner.Save(ctx, id, s, expected)
}

// The kernel memory store serves reads and writes by ID only (memory v1.0.0-rc.5
// store.go:27-33), so every relation a use case queries by another key is a double.
type stubRelation[K comparable, S any] struct {
	step  string
	rec   *serviceskit.Steps
	byKey map[K][]S
	err   error
	asked []K
}

func newStubRelation[K comparable, S any](rec *serviceskit.Steps, step string) *stubRelation[K, S] {
	return &stubRelation[K, S]{step: step, rec: rec, byKey: map[K][]S{}}
}

func (r *stubRelation[K, S]) query(key K) ([]S, error) {
	r.rec.Record(r.step)
	r.asked = append(r.asked, key)
	if r.err != nil {
		return nil, r.err
	}
	return r.byKey[key], nil
}

func (r *stubRelation[K, S]) hold(key K, snapshots ...S) {
	r.byKey[key] = append(r.byKey[key], snapshots...)
}

type partnersOfOrganization struct {
	*stubRelation[domain.OrganizationID, domain.PartnerSnapshot]
}

func (r partnersOfOrganization) LoadByOrganization(_ context.Context, id domain.OrganizationID) ([]domain.PartnerSnapshot, error) {
	return r.query(id)
}

type agreementsOfPartner struct {
	*stubRelation[domain.PartnerID, domain.PartnerAgreementSnapshot]
}

func (r agreementsOfPartner) LoadByPartner(_ context.Context, id domain.PartnerID) ([]domain.PartnerAgreementSnapshot, error) {
	return r.query(id)
}

type profilesOfPartner struct {
	*stubRelation[domain.PartnerID, domain.IntegrationProfileSnapshot]
}

func (r profilesOfPartner) LoadByPartner(_ context.Context, id domain.PartnerID) ([]domain.IntegrationProfileSnapshot, error) {
	return r.query(id)
}

type policiesOfPartner struct {
	*stubRelation[domain.PartnerID, domain.OperationalPolicySnapshot]
}

func (r policiesOfPartner) LoadByPartner(_ context.Context, id domain.PartnerID) ([]domain.OperationalPolicySnapshot, error) {
	return r.query(id)
}

type capabilitiesOfPartner struct {
	*stubRelation[domain.PartnerID, domain.CapabilitySnapshot]
}

func (r capabilitiesOfPartner) LoadByPartner(_ context.Context, id domain.PartnerID) ([]domain.CapabilitySnapshot, error) {
	return r.query(id)
}

type productsOfPartner struct {
	*stubRelation[domain.PartnerID, domain.FinancialProductSnapshot]
}

func (r productsOfPartner) LoadByPartner(_ context.Context, id domain.PartnerID) ([]domain.FinancialProductSnapshot, error) {
	return r.query(id)
}

type partnersOfRole struct {
	*stubRelation[domain.PartnerRole, domain.PartnerSnapshot]
}

func (r partnersOfRole) LoadByRole(_ context.Context, role domain.PartnerRole) ([]domain.PartnerSnapshot, error) {
	return r.query(role)
}

type profilesOfEnvironment struct {
	*stubRelation[domain.Environment, domain.IntegrationProfileSnapshot]
}

func (r profilesOfEnvironment) LoadByEnvironment(_ context.Context, environment domain.Environment) ([]domain.IntegrationProfileSnapshot, error) {
	return r.query(environment)
}

type recordingReader[ID comparable, S any] struct {
	step  string
	rec   *serviceskit.Steps
	inner port.Reader[ID, S]
	err   error
}

func (r *recordingReader[ID, S]) Load(ctx context.Context, id ID) (S, port.Version, error) {
	r.rec.Record(r.step)
	if r.err != nil {
		var zero S
		return zero, 0, r.err
	}
	return r.inner.Load(ctx, id)
}

type fixedBuckets map[string]int

func (b fixedBuckets) Bucket(id domain.CapabilityID, tenant port.TenantID) int {
	return b[string(id)+":"+string(tenant)]
}

type directoryCall struct {
	tenant port.TenantID
	id     domain.OrganizationID
}

type fakeDirectory struct {
	rec   *serviceskit.Steps
	kinds map[domain.OrganizationID][]domain.OrganizationKind
	err   error
	calls []directoryCall
}

func (d *fakeDirectory) FindOrganization(_ context.Context, tenant port.TenantID, id domain.OrganizationID) ([]domain.OrganizationKind, error) {
	d.rec.Record("directory.FindOrganization")
	d.calls = append(d.calls, directoryCall{tenant, id})
	if d.err != nil {
		return nil, d.err
	}
	kinds, found := d.kinds[id]
	if !found {
		return nil, port.ErrNotFound
	}
	return kinds, nil
}

type option func(*setup)

type setup struct {
	registerErr error
	tableFaults map[string]serviceskit.Faults
	outbox      serviceskit.Faults
}

func withTableFaults(table string, faults serviceskit.Faults) option {
	return func(s *setup) { s.tableFaults[table] = faults }
}

func withEnqueueError(err error) option {
	return func(s *setup) { s.outbox.Enqueue = err }
}

func withRegisterError(err error) option {
	return func(s *setup) { s.registerErr = err }
}

func newHarness(t *testing.T, options ...option) *harness {
	t.Helper()

	fakes := serviceskit.NewFakes()
	h := &harness{fakes: fakes, store: fakes.Store, rec: fakes.Steps}
	h.directory = &fakeDirectory{rec: h.rec, kinds: map[domain.OrganizationID][]domain.OrganizationKind{
		testOrganizationID: {domain.KindFunder},
	}}
	h.partners = newStubRelation[domain.OrganizationID, domain.PartnerSnapshot](h.rec, "partners.LoadByOrganization")
	h.agreements = newStubRelation[domain.PartnerID, domain.PartnerAgreementSnapshot](h.rec, "partner_agreements.LoadByPartner")
	h.profiles = newStubRelation[domain.PartnerID, domain.IntegrationProfileSnapshot](h.rec, "integration_profiles.LoadByPartner")
	h.policies = newStubRelation[domain.PartnerID, domain.OperationalPolicySnapshot](h.rec, "operational_policies.LoadByPartner")
	h.caps = newStubRelation[domain.PartnerID, domain.CapabilitySnapshot](h.rec, "capabilities.LoadByPartner")
	h.products = newStubRelation[domain.PartnerID, domain.FinancialProductSnapshot](h.rec, "financial_products.LoadByPartner")
	h.byRole = newStubRelation[domain.PartnerRole, domain.PartnerSnapshot](h.rec, "partners.LoadByRole")
	h.byEnv = newStubRelation[domain.Environment, domain.IntegrationProfileSnapshot](h.rec, "integration_profiles.LoadByEnvironment")
	h.partnerRead = partnerTable.Reader(h.store)
	h.partnerLoad = &recordingReader[domain.PartnerID, domain.PartnerSnapshot]{step: "partners.Read", rec: h.rec, inner: h.partnerRead}
	h.agreeRead = agreementTable.Reader(h.store)
	h.profileRead = profileTable.Reader(h.store)
	h.policyRead = policyTable.Reader(h.store)
	h.capRead = capTable.Reader(h.store)
	h.productRead = productTable.Reader(h.store)
	cfg := &setup{tableFaults: map[string]serviceskit.Faults{}}
	for _, apply := range options {
		apply(cfg)
	}
	fakes.FailRegister = cfg.registerErr

	bind := func(tx serviceskit.Tx) application.Resources {
		return application.Resources{
			Partners: recordingRepo[domain.PartnerID, domain.PartnerSnapshot]{
				table: "partners", rec: h.rec,
				inner: serviceskit.FaultyRepository(partnerTable.Repository(tx.Memory()), cfg.tableFaults["partners"]),
			},
			Agreements: recordingRepo[domain.AgreementID, domain.PartnerAgreementSnapshot]{
				table: "partner_agreements", rec: h.rec,
				inner: serviceskit.FaultyRepository(agreementTable.Repository(tx.Memory()), cfg.tableFaults["partner_agreements"]),
			},
			IntegrationProfiles: recordingRepo[domain.ProfileID, domain.IntegrationProfileSnapshot]{
				table: "integration_profiles", rec: h.rec,
				inner: serviceskit.FaultyRepository(profileTable.Repository(tx.Memory()), cfg.tableFaults["integration_profiles"]),
			},
			OperationalPolicies: recordingRepo[domain.PolicyID, domain.OperationalPolicySnapshot]{
				table: "operational_policies", rec: h.rec,
				inner: serviceskit.FaultyRepository(policyTable.Repository(tx.Memory()), cfg.tableFaults["operational_policies"]),
			},
			Capabilities: recordingRepo[domain.CapabilityID, domain.CapabilitySnapshot]{
				table: "capabilities", rec: h.rec,
				inner: serviceskit.FaultyRepository(capTable.Repository(tx.Memory()), cfg.tableFaults["capabilities"]),
			},
			FinancialProducts: recordingRepo[domain.ProductID, domain.FinancialProductSnapshot]{
				table: "financial_products", rec: h.rec,
				inner: serviceskit.FaultyRepository(productTable.Repository(tx.Memory()), cfg.tableFaults["financial_products"]),
			},
			AgreementsByPartner:   agreementsOfPartner{h.agreements},
			ProfilesByPartner:     profilesOfPartner{h.profiles},
			CapabilitiesByPartner: capabilitiesOfPartner{h.caps},
			Outbox:                serviceskit.FaultyOutbox(tx.Outbox(), cfg.outbox),
			Commands:              tx.CommandInbox(application.CommandConsumer),
		}
	}

	h.service = application.Service{
		UoW:                    serviceskit.UnitOfWork(fakes, bind),
		Directory:              h.directory,
		PartnersByOrganization: partnersOfOrganization{h.partners},
		AgreementsByPartner:    agreementsOfPartner{h.agreements},
		ProfilesByPartner:      profilesOfPartner{h.profiles},
		PoliciesByPartner:      policiesOfPartner{h.policies},
		CapabilitiesByPartner:  capabilitiesOfPartner{h.caps},
		ProductsByPartner:      productsOfPartner{h.products},
		PartnerReader:          h.partnerLoad,
		PartnersByRole:         partnersOfRole{h.byRole},
		ProfilesByEnvironment:  profilesOfEnvironment{h.byEnv},
		Rollout:                fixedBuckets{},
		Clock:                  fakes.Clock(memory.FixedClock{At: testOccurred}),
		IDs:                    fakes.IDs(&memory.SequenceIDs{Prefix: "m-"}),
		Authorize:              serviceskit.Authorize(fakes, usecase.AllowAll[application.Operation]()),
		Idempotency:            usecase.IdempotencyPolicy{Wait: 1_000_000_000, Retention: 86_400_000_000_000, Digest: serviceskit.FoldDigest},
	}
	return h
}

func seed[ID comparable, S any](t *testing.T, h *harness, ctx context.Context, table memory.Table[ID, S], id ID, s S) {
	t.Helper()
	uow := memory.NewUnitOfWork(h.store, func(tx *memory.Tx) port.Repository[ID, S] { return table.Repository(tx) })
	err := uow.Within(ctx, func(ctx context.Context, repo port.Repository[ID, S]) error {
		return repo.Save(ctx, id, s, 0)
	})
	if err != nil {
		t.Fatalf("seed %s %v: %v", table.Name, id, err)
	}
	h.fakes.MarkSeeded()
}

func (h *harness) outboxLen() int {
	return len(h.store.Entries())
}

func registerPartner() application.RegisterPartner {
	return application.RegisterPartner{PartnerID: testPartnerID, OrganizationID: testOrganizationID, Role: domain.RoleFunder}
}

func registerAgreement() application.RegisterAgreement {
	return application.RegisterAgreement{
		AgreementID:          testAgreementID,
		PartnerID:            testPartnerID,
		ValidFrom:            kernel.Instant(testOccurred),
		ValidUntil:           kernel.Instant(testOccurred) + 365*day,
		InstrumentTemplateID: "tpl-1",
	}
}

func activateAgreement() application.ActivateAgreement {
	return application.ActivateAgreement{
		AgreementID:        testAgreementID,
		ContractEvidenceID: "evd-1",
		SatisfiedCriteria:  []string{"H14", "H15", "H17", "H18"},
	}
}

func draftAgreement(id domain.AgreementID, version int, supersedes domain.AgreementID) domain.PartnerAgreementSnapshot {
	return domain.PartnerAgreementSnapshot{
		ID:           id,
		PartnerID:    testPartnerID,
		Version:      version,
		SupersedesID: supersedes,
		Status:       domain.AgreementDraft,
		ValidFrom:    kernel.Instant(testOccurred),
		ValidUntil:   kernel.Instant(testOccurred) + 365*day,
	}
}

func registerProfile() application.RegisterIntegrationProfile {
	return application.RegisterIntegrationProfile{
		ProfileID:            testProfileID,
		PartnerID:            testPartnerID,
		Environment:          domain.EnvironmentProduction,
		ChangeKind:           domain.ChangeInitial,
		Protocols:            []string{"https", "mtls"},
		Endpoint:             "https://partner.example/api",
		SchemaVersion:        "v3",
		SecretRef:            "vault://partners/prt-100/api-key",
		CertificateRef:       "cert-100",
		CertificateExpiresAt: kernel.Instant(testOccurred) + 90*day,
		EffectiveFrom:        kernel.Instant(testOccurred),
	}
}

func profileVersion(id domain.ProfileID, environment domain.Environment, version int, supersedes domain.ProfileID, expiresIn kernel.Instant) domain.IntegrationProfileSnapshot {
	return domain.IntegrationProfileSnapshot{
		ID:                   id,
		PartnerID:            testPartnerID,
		Environment:          environment,
		Version:              version,
		SupersedesID:         supersedes,
		ChangeKind:           domain.ChangeInitial,
		Protocols:            []string{"https"},
		Endpoint:             "https://partner.example/api",
		SecretRef:            "vault://partners/prt-100/api-key",
		CertificateExpiresAt: kernel.Instant(testOccurred) + expiresIn,
		EffectiveFrom:        kernel.Instant(testOccurred),
	}
}

func raiseAlert(thresholdDays int) application.RaiseCertificateExpiryAlert {
	return application.RaiseCertificateExpiryAlert{ProfileID: testProfileID, ThresholdDays: thresholdDays}
}

func definePolicy() application.DefineOperationalPolicy {
	return application.DefineOperationalPolicy{
		PolicyID:           testPolicyID,
		PartnerID:          testPartnerID,
		EffectiveFrom:      kernel.Instant(testOccurred),
		RateLimitPerMinute: 100,
		BusinessDays:       []string{"mon", "tue", "wed", "thu", "fri"},
		CutoffTime:         "17:00",
		ProcessingWindow:   "08:00-18:00",
		SlaRef:             "sla-gold",
		Fallback:           "manual queue",
		SupportContact:     "support@partner.example",
		IncidentContact:    "+55 11 99999-0000",
	}
}

func policyVersion(id domain.PolicyID, version int, supersedes domain.PolicyID) domain.OperationalPolicySnapshot {
	return domain.OperationalPolicySnapshot{
		ID: id, PartnerID: testPartnerID, Version: version, SupersedesID: supersedes, EffectiveFrom: kernel.Instant(testOccurred),
		RateLimitPerMinute: 100, SupportContact: "support@partner.example", IncidentContact: "+55 11 99999-0000",
	}
}
