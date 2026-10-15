package application_test

import (
	"context"
	"errors"
	"testing"

	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/memory"
	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/serviceskit"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/application"
	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/ports"
)

const (
	testOrganizationID = domain.OrganizationID("org-100")
	testJurisdiction   = "BR"
	testLegalName      = "Recebiveis Sinteticos Ltda"
	testTradeName      = "Sintetica"
	testTenant         = port.TenantID("acme")
)

var testOccurred = port.Instant(1_755_432_000_000_000_000)

var organizationTable = memory.Table[domain.OrganizationID, domain.OrganizationSnapshot]{Name: "organizations"}

var (
	establishmentTable = memory.Table[domain.EstablishmentID, domain.EstablishmentSnapshot]{
		Name: "establishments",
		Clone: func(s domain.EstablishmentSnapshot) domain.EstablishmentSnapshot {
			return domain.FromEstablishmentSnapshot(s).Snapshot()
		},
	}
	personTable         = memory.Table[domain.PersonID, domain.PersonSnapshot]{Name: "persons"}
	representationTable = memory.Table[domain.RepresentationID, domain.RepresentationSnapshot]{
		Name: "representations",
		Clone: func(s domain.RepresentationSnapshot) domain.RepresentationSnapshot {
			return domain.FromRepresentationSnapshot(s).Snapshot()
		},
	}
	userAccountTable = memory.Table[domain.UserID, domain.UserAccountSnapshot]{Name: "user_accounts"}
	membershipTable  = memory.Table[domain.MembershipID, domain.MembershipSnapshot]{Name: "memberships"}

	bankAccountChangeTable = memory.Table[domain.BankAccountChangeID, domain.BankAccountChangeSnapshot]{Name: "bank_account_changes"}
)

type harness struct {
	fakes     *serviceskit.Fakes
	store     *memory.Store
	rec       *serviceskit.Steps
	protector *fakeProtector
	taxIDs    *stubTaxIDReader
	service   application.Service

	establishmentTaxIDs  *stubRelation[string, domain.EstablishmentSnapshot]
	establishmentsByOrg  *stubRelation[domain.OrganizationID, domain.EstablishmentSnapshot]
	personDocuments      *stubRelation[string, domain.PersonSnapshot]
	representationsByOrg *stubRelation[domain.OrganizationID, domain.RepresentationSnapshot]
	memberships          *trackedMemberships
	accounts             *recordingReader[domain.UserID, domain.UserAccountSnapshot]

	bankAccountChangesByOrg *stubRelation[domain.OrganizationID, domain.BankAccountChangeSnapshot]
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

// The kernel memory store serves reads and writes by ID only (memory v1.0.0-rc.4
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

type establishmentTaxIDs struct {
	*stubRelation[string, domain.EstablishmentSnapshot]
}

func (r establishmentTaxIDs) LoadByTaxIDHash(_ context.Context, taxIDHash string) ([]domain.EstablishmentSnapshot, error) {
	return r.query(taxIDHash)
}

type establishmentsOfOrganization struct {
	*stubRelation[domain.OrganizationID, domain.EstablishmentSnapshot]
}

func (r establishmentsOfOrganization) LoadByOrganization(_ context.Context, id domain.OrganizationID) ([]domain.EstablishmentSnapshot, error) {
	return r.query(id)
}

type personDocuments struct {
	*stubRelation[string, domain.PersonSnapshot]
}

func (r personDocuments) LoadByDocumentHash(_ context.Context, documentHash string) ([]domain.PersonSnapshot, error) {
	return r.query(documentHash)
}

type representationsOfOrganization struct {
	*stubRelation[domain.OrganizationID, domain.RepresentationSnapshot]
}

func (r representationsOfOrganization) LoadByOrganization(_ context.Context, id domain.OrganizationID) ([]domain.RepresentationSnapshot, error) {
	return r.query(id)
}

type trackedMemberships struct {
	rec   *serviceskit.Steps
	store *memory.Store
	ids   []domain.MembershipID
	err   error
}

func (r *trackedMemberships) track(ids ...domain.MembershipID) {
	r.ids = append(r.ids, ids...)
}

func (r *trackedMemberships) LoadByUser(ctx context.Context, user domain.UserID) ([]domain.MembershipSnapshot, error) {
	r.rec.Record("memberships.LoadByUser")
	return r.filter(ctx, func(s domain.MembershipSnapshot) bool { return s.UserID == user })
}

func (r *trackedMemberships) LoadByOrganization(ctx context.Context, organization domain.OrganizationID) ([]domain.MembershipSnapshot, error) {
	r.rec.Record("memberships.LoadByOrganization")
	return r.filter(ctx, func(s domain.MembershipSnapshot) bool { return s.OrganizationID == organization })
}

func (r *trackedMemberships) filter(ctx context.Context, keep func(domain.MembershipSnapshot) bool) ([]domain.MembershipSnapshot, error) {
	if r.err != nil {
		return nil, r.err
	}
	var found []domain.MembershipSnapshot
	for _, id := range r.ids {
		s, _, err := membershipTable.Reader(r.store).Load(ctx, id)
		switch {
		case errors.Is(err, port.ErrNotFound):
			continue
		case err != nil:
			return nil, err
		case keep(s):
			found = append(found, s)
		}
	}
	return found, nil
}

type bankAccountChangesOfOrganization struct {
	*stubRelation[domain.OrganizationID, domain.BankAccountChangeSnapshot]
}

func (r bankAccountChangesOfOrganization) LoadByOrganization(_ context.Context, id domain.OrganizationID) ([]domain.BankAccountChangeSnapshot, error) {
	return r.query(id)
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

type recordingOrganizationRepo struct {
	inner ports.OrganizationRepository
	rec   *serviceskit.Steps
}

func (r recordingOrganizationRepo) Load(ctx context.Context, id domain.OrganizationID) (domain.OrganizationSnapshot, port.Version, error) {
	r.rec.Record("organizations.Load")
	return r.inner.Load(ctx, id)
}

func (r recordingOrganizationRepo) Save(ctx context.Context, id domain.OrganizationID, s domain.OrganizationSnapshot, expected port.Version) error {
	r.rec.Record("organizations.Save")
	return r.inner.Save(ctx, id, s, expected)
}

type fakeProtector struct {
	rec   *serviceskit.Steps
	err   error
	calls []protectCall
}

type protectCall struct {
	tenant port.TenantID
	kind   domain.TaxIDType
	value  string
}

func (p *fakeProtector) Protect(_ context.Context, tenant port.TenantID, kind domain.TaxIDType, value string) (ports.ProtectedIdentifier, error) {
	p.rec.Record("protector.Protect")
	p.calls = append(p.calls, protectCall{tenant, kind, value})
	if p.err != nil {
		return ports.ProtectedIdentifier{}, p.err
	}
	return ports.ProtectedIdentifier{Hash: fakeHash(tenant, kind, value), KeyVersion: 1}, nil
}

func fakeHash(tenant port.TenantID, kind domain.TaxIDType, value string) string {
	return string(tenant) + "|" + string(kind) + "|" + domain.NormalizeTaxID(kind, value)
}

type stubTaxIDReader struct {
	rec    *serviceskit.Steps
	byHash map[string][]domain.OrganizationSnapshot
	err    error
	asked  []string
}

func (r *stubTaxIDReader) LoadByTaxIDHash(_ context.Context, taxIDHash string) ([]domain.OrganizationSnapshot, error) {
	r.rec.Record("organizations.LoadByTaxIDHash")
	r.asked = append(r.asked, taxIDHash)
	if r.err != nil {
		return nil, r.err
	}
	return r.byHash[taxIDHash], nil
}

func (r *stubTaxIDReader) hold(snapshots ...domain.OrganizationSnapshot) {
	for _, s := range snapshots {
		r.byHash[s.TaxIDHash] = append(r.byHash[s.TaxIDHash], s)
	}
}

type option func(*setup)

type setup struct {
	faults      serviceskit.Faults
	registerErr error
	tableFaults map[string]serviceskit.Faults
}

func withTableFaults(table string, faults serviceskit.Faults) option {
	return func(s *setup) { s.tableFaults[table] = faults }
}

func withRegisterError(err error) option {
	return func(s *setup) { s.registerErr = err }
}

func withLoadError(err error) option {
	return func(s *setup) { s.faults.Load = err }
}

func withSaveError(err error) option {
	return func(s *setup) { s.faults.Save = err }
}

func withEnqueueError(err error) option {
	return func(s *setup) { s.faults.Enqueue = err }
}

func newHarness(t *testing.T, options ...option) *harness {
	t.Helper()

	fakes := serviceskit.NewFakes()
	h := &harness{fakes: fakes, store: fakes.Store, rec: fakes.Steps}
	h.protector = &fakeProtector{rec: h.rec}
	h.taxIDs = &stubTaxIDReader{rec: h.rec, byHash: map[string][]domain.OrganizationSnapshot{}}
	h.establishmentTaxIDs = newStubRelation[string, domain.EstablishmentSnapshot](h.rec, "establishments.LoadByTaxIDHash")
	h.establishmentsByOrg = newStubRelation[domain.OrganizationID, domain.EstablishmentSnapshot](h.rec, "establishments.LoadByOrganization")
	h.personDocuments = newStubRelation[string, domain.PersonSnapshot](h.rec, "persons.LoadByDocumentHash")
	h.representationsByOrg = newStubRelation[domain.OrganizationID, domain.RepresentationSnapshot](h.rec, "representations.LoadByOrganization")
	h.memberships = &trackedMemberships{rec: h.rec, store: h.store}
	h.accounts = &recordingReader[domain.UserID, domain.UserAccountSnapshot]{step: "user_accounts.Read", rec: h.rec, inner: userAccountTable.Reader(h.store)}
	h.bankAccountChangesByOrg = newStubRelation[domain.OrganizationID, domain.BankAccountChangeSnapshot](h.rec, "bank_account_changes.LoadByOrganization")
	cfg := &setup{tableFaults: map[string]serviceskit.Faults{}}
	for _, apply := range options {
		apply(cfg)
	}
	fakes.FailRegister = cfg.registerErr

	bind := func(tx serviceskit.Tx) application.Resources {
		return application.Resources{
			Organizations: recordingOrganizationRepo{inner: serviceskit.FaultyRepository(organizationTable.Repository(tx.Memory()), cfg.faults), rec: h.rec},
			Establishments: recordingRepo[domain.EstablishmentID, domain.EstablishmentSnapshot]{
				table: "establishments", rec: h.rec,
				inner: serviceskit.FaultyRepository(establishmentTable.Repository(tx.Memory()), cfg.tableFaults["establishments"]),
			},
			Persons: recordingRepo[domain.PersonID, domain.PersonSnapshot]{
				table: "persons", rec: h.rec,
				inner: serviceskit.FaultyRepository(personTable.Repository(tx.Memory()), cfg.tableFaults["persons"]),
			},
			Representations: recordingRepo[domain.RepresentationID, domain.RepresentationSnapshot]{
				table: "representations", rec: h.rec,
				inner: serviceskit.FaultyRepository(representationTable.Repository(tx.Memory()), cfg.tableFaults["representations"]),
			},
			UserAccounts: recordingRepo[domain.UserID, domain.UserAccountSnapshot]{
				table: "user_accounts", rec: h.rec,
				inner: serviceskit.FaultyRepository(userAccountTable.Repository(tx.Memory()), cfg.tableFaults["user_accounts"]),
			},
			Memberships: recordingRepo[domain.MembershipID, domain.MembershipSnapshot]{
				table: "memberships", rec: h.rec,
				inner: serviceskit.FaultyRepository(membershipTable.Repository(tx.Memory()), cfg.tableFaults["memberships"]),
			},
			BankAccountChanges: recordingRepo[domain.BankAccountChangeID, domain.BankAccountChangeSnapshot]{
				table: "bank_account_changes", rec: h.rec,
				inner: serviceskit.FaultyRepository(bankAccountChangeTable.Repository(tx.Memory()), cfg.tableFaults["bank_account_changes"]),
			},
			Outbox:   serviceskit.FaultyOutbox(tx.Outbox(), cfg.faults),
			Commands: tx.CommandInbox(application.CommandConsumer),
		}
	}

	h.service = application.Service{
		UoW:         serviceskit.UnitOfWork(fakes, bind),
		Reader:      organizationTable.Reader(h.store),
		TaxIDReader: h.taxIDs,

		EstablishmentTaxIDReader:      establishmentTaxIDs{h.establishmentTaxIDs},
		EstablishmentsByOrganization:  establishmentsOfOrganization{h.establishmentsByOrg},
		PersonReader:                  personTable.Reader(h.store),
		DocumentReader:                personDocuments{h.personDocuments},
		RepresentationReader:          representationTable.Reader(h.store),
		RepresentationsByOrganization: representationsOfOrganization{h.representationsByOrg},
		UserAccountReader:             h.accounts,
		MembershipsByUser:             h.memberships,
		MembershipsByOrganization:     h.memberships,
		Protector:                     h.protector,
		Clock:                         fakes.Clock(memory.FixedClock{At: testOccurred}),
		IDs:                           fakes.IDs(&memory.SequenceIDs{Prefix: "m-"}),
		Authorize:                     serviceskit.Authorize(fakes, usecase.AllowAll[application.Operation]()),
		Idempotency:                   usecase.IdempotencyPolicy{Wait: 1_000_000_000, Retention: 86_400_000_000_000, Digest: serviceskit.FoldDigest},

		BankAccountChangeReader:          bankAccountChangeTable.Reader(h.store),
		BankAccountChangesByOrganization: bankAccountChangesOfOrganization{h.bankAccountChangesByOrg},
	}
	return h
}

func (h *harness) seedOrganization(t *testing.T, ctx context.Context, snap domain.OrganizationSnapshot) {
	t.Helper()
	uow := memory.NewUnitOfWork(h.store, func(tx *memory.Tx) application.Resources {
		return application.Resources{Organizations: organizationTable.Repository(tx)}
	})
	err := uow.Within(ctx, func(ctx context.Context, res application.Resources) error {
		return res.Organizations.Save(ctx, snap.ID, snap, 0)
	})
	if err != nil {
		t.Fatalf("seedOrganization: %v", err)
	}
	h.fakes.MarkSeeded()
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

func register() application.RegisterOrganization {
	return application.RegisterOrganization{
		OrganizationID: testOrganizationID,
		Kinds:          []domain.OrganizationKind{domain.KindClient},
		Jurisdiction:   testJurisdiction,
		TaxIDType:      domain.TaxIDTypeCNPJ,
		TaxID:          alphanumericCNPJ,
		LegalName:      testLegalName,
		TradeName:      testTradeName,
	}
}

func registeredSnapshot(id domain.OrganizationID, tenant port.TenantID, jurisdiction string) domain.OrganizationSnapshot {
	return domain.OrganizationSnapshot{
		ID:                  id,
		Kinds:               domain.KindsOf(domain.KindClient),
		Jurisdiction:        jurisdiction,
		TaxIDType:           domain.TaxIDTypeCNPJ,
		TaxID:               alphanumericCNPJ,
		TaxIDHash:           fakeHash(tenant, domain.TaxIDTypeCNPJ, alphanumericCNPJ),
		TaxIDHashKeyVersion: 1,
		TaxIDMasked:         domain.MaskTaxID(domain.TaxIDTypeCNPJ, alphanumericCNPJ),
		LegalName:           testLegalName,
		Status:              domain.OrganizationActive,
		CreatedAt:           testOccurred,
	}
}
