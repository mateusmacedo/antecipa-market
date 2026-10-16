package provider_test

import (
	"errors"
	"slices"
	"testing"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	eventv1 "github.com/mateusmacedo/antecipa-market/apps/backend/identity/contract/gen/go/company/identity/event/v1"
	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/provider"
)

const (
	organizationRegisteredType = "com.company.identity.organization-registered.v1"
	establishmentAddedType     = "com.company.identity.establishment-added.v1"
	membershipGrantedType      = "com.company.identity.membership-granted.v1"
	membershipRevokedType      = "com.company.identity.membership-revoked.v1"
	userSuspendedType          = "com.company.identity.user-suspended.v1"

	bankAccountChangeRequestedType = "com.company.identity.bank-account-change-requested.v1"
	bankAccountChangeApprovedType  = "com.company.identity.bank-account-change-approved.v1"
)

type strayEvent struct{}

func (strayEvent) EventName() string { return "identity-organization.stray" }

var _ kernel.DomainEvent = strayEvent{}

func TestMapperMapsOrganizationRegisteredWithHashAndMask(t *testing.T) {
	t.Parallel()

	event := domain.OrganizationRegistered{
		OrganizationID: "org-100",
		Kinds:          domain.KindsOf(domain.KindFunder, domain.KindClient),
		Jurisdiction:   "BR",
		TaxIDType:      domain.TaxIDTypeCNPJ,
		TaxIDHash:      "4a455609da3a3d6d689b65257f38532e3868396abc39d3b92311b18489f169dc",
		TaxIDMasked:    "**.***.***/**DE-35",
		At:             1_755_432_000_123_456_789,
	}

	mapped, err := provider.Mapper{}.Map(event)

	if err != nil {
		t.Fatalf("Map() = %v, want nil", err)
	}
	want := &eventv1.OrganizationRegistered{
		OrganizationId: "org-100",
		Kinds:          []eventv1.OrganizationKind{eventv1.OrganizationKind_ORGANIZATION_KIND_CLIENT, eventv1.OrganizationKind_ORGANIZATION_KIND_FUNDER},
		Jurisdiction:   "BR",
		TaxIdType:      eventv1.TaxIdType_TAX_ID_TYPE_CNPJ,
		TaxIdHash:      "4a455609da3a3d6d689b65257f38532e3868396abc39d3b92311b18489f169dc",
		TaxIdMasked:    "**.***.***/**DE-35",
		RegisteredAt:   &timestamppb.Timestamp{Seconds: 1_755_432_000, Nanos: 123_456_789},
	}
	if !proto.Equal(mapped.Message, want) {
		t.Errorf("Map().Message = %v, want %v", mapped.Message, want)
	}
	if mapped.Type != organizationRegisteredType {
		t.Errorf("Map().Type = %q, want %q (PTB-03)", mapped.Type, organizationRegisteredType)
	}
}

func TestMapperMapsEachTaxIDTypeAndEachKind(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		kind     domain.OrganizationKind
		taxType  domain.TaxIDType
		wantKind eventv1.OrganizationKind
		wantType eventv1.TaxIdType
	}{
		{"client under CNPJ", domain.KindClient, domain.TaxIDTypeCNPJ, eventv1.OrganizationKind_ORGANIZATION_KIND_CLIENT, eventv1.TaxIdType_TAX_ID_TYPE_CNPJ},
		{"funder under CPF", domain.KindFunder, domain.TaxIDTypeCPF, eventv1.OrganizationKind_ORGANIZATION_KIND_FUNDER, eventv1.TaxIdType_TAX_ID_TYPE_CPF},
		{"partner", domain.KindPartner, domain.TaxIDTypeCNPJ, eventv1.OrganizationKind_ORGANIZATION_KIND_PARTNER, eventv1.TaxIdType_TAX_ID_TYPE_CNPJ},
		{"platform", domain.KindPlatform, domain.TaxIDTypeCNPJ, eventv1.OrganizationKind_ORGANIZATION_KIND_PLATFORM, eventv1.TaxIdType_TAX_ID_TYPE_CNPJ},
	} {
		mapped, err := provider.Mapper{}.Map(domain.OrganizationRegistered{
			OrganizationID: "org-1", Kinds: domain.KindsOf(tc.kind), Jurisdiction: "BR", TaxIDType: tc.taxType, At: 1,
		})
		if err != nil {
			t.Fatalf("%s: Map() = %v, want nil", tc.name, err)
		}
		got, ok := mapped.Message.(*eventv1.OrganizationRegistered)
		if !ok {
			t.Fatalf("%s: Map().Message is %T, want *eventv1.OrganizationRegistered", tc.name, mapped.Message)
		}
		if len(got.GetKinds()) != 1 || got.GetKinds()[0] != tc.wantKind || got.GetTaxIdType() != tc.wantType {
			t.Errorf("%s: kinds %v and type %v, want [%v] and %v", tc.name, got.GetKinds(), got.GetTaxIdType(), tc.wantKind, tc.wantType)
		}
	}
}

// The contract is what keeps the tax id inside the context: if a tax_id field
// ever appeared in the event, the mapper could start publishing it in clear.
func TestTheEventContractCarriesNoTaxIDInClear(t *testing.T) {
	t.Parallel()

	fields := (&eventv1.OrganizationRegistered{}).ProtoReflect().Descriptor().Fields()
	for i := range fields.Len() {
		if name := fields.Get(i).Name(); name == "tax_id" || name == "legal_name" || name == "trade_name" {
			t.Fatalf("OrganizationRegistered declares %s, want only the hash and the mask of the PII to cross the context", name)
		}
	}
	if fields.ByName("tax_id_hash") == nil || fields.ByName("tax_id_masked") == nil {
		t.Fatal("OrganizationRegistered lacks tax_id_hash or tax_id_masked")
	}
}

func TestMapperReportsUnmappedEvent(t *testing.T) {
	t.Parallel()

	_, err := provider.Mapper{}.Map(strayEvent{})

	if !errors.Is(err, postgres.ErrUnmappedEvent) {
		t.Fatalf("Map() = %v, want ErrUnmappedEvent", err)
	}
}

func TestMapperMapsEstablishmentAddedWithTheHashOnly(t *testing.T) {
	t.Parallel()

	event := domain.EstablishmentAdded{
		EstablishmentID: "est-100",
		OrganizationID:  "org-100",
		TaxIDType:       domain.TaxIDTypeCNPJ,
		TaxIDHash:       "9c1185a5c5e9fc54612808977ee8f548b2258d31b6e2e9a4e5a3ab4c2c4e0f11",
		Receiver:        true,
		At:              1_755_432_000_123_456_789,
	}

	mapped, err := provider.Mapper{}.Map(event)

	if err != nil {
		t.Fatalf("Map() = %v, want nil", err)
	}
	want := &eventv1.EstablishmentAdded{
		EstablishmentId: "est-100",
		OrganizationId:  "org-100",
		TaxIdType:       eventv1.TaxIdType_TAX_ID_TYPE_CNPJ,
		TaxIdHash:       "9c1185a5c5e9fc54612808977ee8f548b2258d31b6e2e9a4e5a3ab4c2c4e0f11",
		Receiver:        true,
		AddedAt:         &timestamppb.Timestamp{Seconds: 1_755_432_000, Nanos: 123_456_789},
	}
	if !proto.Equal(mapped.Message, want) {
		t.Errorf("Map().Message = %v, want %v", mapped.Message, want)
	}
	if mapped.Type != establishmentAddedType {
		t.Errorf("Map().Type = %q, want %q (PTB-03)", mapped.Type, establishmentAddedType)
	}
}

func TestMapperMapsANonReceiverEstablishmentUnderCPF(t *testing.T) {
	t.Parallel()

	mapped, err := provider.Mapper{}.Map(domain.EstablishmentAdded{EstablishmentID: "est-1", OrganizationID: "org-1", TaxIDType: domain.TaxIDTypeCPF, At: 1})

	if err != nil {
		t.Fatalf("Map() = %v, want nil", err)
	}
	got, ok := mapped.Message.(*eventv1.EstablishmentAdded)
	if !ok {
		t.Fatalf("Map().Message is %T, want *eventv1.EstablishmentAdded", mapped.Message)
	}
	if got.GetTaxIdType() != eventv1.TaxIdType_TAX_ID_TYPE_CPF || got.GetReceiver() {
		t.Errorf("Map() = %v, want CPF and not a receiver", got)
	}
}

// The tax id of an establishment crosses the context only as its hash (spec,
// EstablishmentAdded): not even the mask is a field of the event.
func TestTheEstablishmentAddedContractCarriesOnlyTheHashOfTheTaxID(t *testing.T) {
	t.Parallel()

	fields := (&eventv1.EstablishmentAdded{}).ProtoReflect().Descriptor().Fields()
	for i := range fields.Len() {
		if name := fields.Get(i).Name(); name == "tax_id" || name == "tax_id_masked" || name == "trade_name" || name == "external_references" {
			t.Fatalf("EstablishmentAdded declares %s, want only the hash of the tax id to cross the context", name)
		}
	}
	if fields.ByName("tax_id_hash") == nil {
		t.Fatal("EstablishmentAdded lacks tax_id_hash")
	}
}

func TestMapperMapsMembershipGrantedWithTheRoleAndTheLevel(t *testing.T) {
	t.Parallel()

	event := domain.MembershipGranted{
		MembershipID:      "mbr-100",
		OrganizationID:    "org-100",
		UserID:            "sub-100",
		Role:              domain.RoleOperator,
		VerificationLevel: domain.VerificationPersonalDataChecked,
		At:                1_755_432_000_123_456_789,
	}

	mapped, err := provider.Mapper{}.Map(event)

	if err != nil {
		t.Fatalf("Map() = %v, want nil", err)
	}
	want := &eventv1.MembershipGranted{
		MembershipId:      "mbr-100",
		OrganizationId:    "org-100",
		UserId:            "sub-100",
		Role:              eventv1.MembershipRole_MEMBERSHIP_ROLE_OPERATOR,
		VerificationLevel: eventv1.VerificationLevel_VERIFICATION_LEVEL_PERSONAL_DATA_CHECKED,
		GrantedAt:         &timestamppb.Timestamp{Seconds: 1_755_432_000, Nanos: 123_456_789},
	}
	if !proto.Equal(mapped.Message, want) {
		t.Errorf("Map().Message = %v, want %v", mapped.Message, want)
	}
	if mapped.Type != membershipGrantedType {
		t.Errorf("Map().Type = %q, want %q (PTB-03)", mapped.Type, membershipGrantedType)
	}
}

func TestMapperMapsEachRoleAndEachVerificationLevel(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		role      domain.Role
		level     domain.VerificationLevel
		wantRole  eventv1.MembershipRole
		wantLevel eventv1.VerificationLevel
	}{
		{domain.RoleOperator, domain.VerificationPersonalDataChecked, eventv1.MembershipRole_MEMBERSHIP_ROLE_OPERATOR, eventv1.VerificationLevel_VERIFICATION_LEVEL_PERSONAL_DATA_CHECKED},
		{domain.RoleAuditor, domain.VerificationChannelOwnership, eventv1.MembershipRole_MEMBERSHIP_ROLE_AUDITOR, eventv1.VerificationLevel_VERIFICATION_LEVEL_CHANNEL_OWNERSHIP},
		{"approver", "document_and_liveness", eventv1.MembershipRole_MEMBERSHIP_ROLE_UNSPECIFIED, eventv1.VerificationLevel_VERIFICATION_LEVEL_UNSPECIFIED},
	} {
		mapped, err := provider.Mapper{}.Map(domain.MembershipGranted{MembershipID: "mbr-1", Role: tc.role, VerificationLevel: tc.level, At: 1})
		if err != nil {
			t.Fatalf("%s: Map() = %v, want nil", tc.role, err)
		}
		got, ok := mapped.Message.(*eventv1.MembershipGranted)
		if !ok {
			t.Fatalf("%s: Map().Message is %T, want *eventv1.MembershipGranted", tc.role, mapped.Message)
		}
		if got.GetRole() != tc.wantRole || got.GetVerificationLevel() != tc.wantLevel {
			t.Errorf("%s: role %v and level %v, want %v and %v", tc.role, got.GetRole(), got.GetVerificationLevel(), tc.wantRole, tc.wantLevel)
		}
	}
}

func TestMapperMapsMembershipRevokedWithTheReason(t *testing.T) {
	t.Parallel()

	event := domain.MembershipRevoked{
		MembershipID:   "mbr-100",
		OrganizationID: "org-100",
		UserID:         "sub-100",
		Role:           domain.RoleAuditor,
		Reason:         "left the company",
		At:             1_755_432_000_123_456_789,
	}

	mapped, err := provider.Mapper{}.Map(event)

	if err != nil {
		t.Fatalf("Map() = %v, want nil", err)
	}
	want := &eventv1.MembershipRevoked{
		MembershipId:   "mbr-100",
		OrganizationId: "org-100",
		UserId:         "sub-100",
		Role:           eventv1.MembershipRole_MEMBERSHIP_ROLE_AUDITOR,
		Reason:         "left the company",
		RevokedAt:      &timestamppb.Timestamp{Seconds: 1_755_432_000, Nanos: 123_456_789},
	}
	if !proto.Equal(mapped.Message, want) {
		t.Errorf("Map().Message = %v, want %v", mapped.Message, want)
	}
	if mapped.Type != membershipRevokedType {
		t.Errorf("Map().Type = %q, want %q (PTB-03)", mapped.Type, membershipRevokedType)
	}
}

func TestMapperMapsUserSuspendedWithTheReason(t *testing.T) {
	t.Parallel()

	mapped, err := provider.Mapper{}.Map(domain.UserSuspended{UserID: "sub-100", Reason: "fraud review", At: 1_755_432_000_123_456_789})

	if err != nil {
		t.Fatalf("Map() = %v, want nil", err)
	}
	want := &eventv1.UserSuspended{
		UserId:      "sub-100",
		Reason:      "fraud review",
		SuspendedAt: &timestamppb.Timestamp{Seconds: 1_755_432_000, Nanos: 123_456_789},
	}
	if !proto.Equal(mapped.Message, want) {
		t.Errorf("Map().Message = %v, want %v", mapped.Message, want)
	}
	if mapped.Type != userSuspendedType {
		t.Errorf("Map().Type = %q, want %q (PTB-03)", mapped.Type, userSuspendedType)
	}
}

// The invitee email is personal data of the membership: no event of the
// membership may declare it, or the mapper could start publishing it.
func TestNoMembershipEventContractCarriesTheInviteeEmail(t *testing.T) {
	t.Parallel()

	for _, message := range []proto.Message{&eventv1.MembershipGranted{}, &eventv1.MembershipRevoked{}} {
		fields := message.ProtoReflect().Descriptor().Fields()
		for i := range fields.Len() {
			if fields.Get(i).Name() == "invitee_email" {
				t.Fatalf("%s declares invitee_email, want the membership to cross the context without it", message.ProtoReflect().Descriptor().Name())
			}
		}
	}
}

func TestMapperMapsBankAccountChangeRequestedWithTheEvidenceAndTheRequester(t *testing.T) {
	t.Parallel()

	event := domain.BankAccountChangeRequested{
		BankAccountChangeID:   "bac-100",
		OrganizationID:        "org-100",
		DestinationEvidenceID: "evd-100",
		RequestedByUserID:     "sub-100",
		At:                    1_755_432_000_123_456_789,
	}

	mapped, err := provider.Mapper{}.Map(event)

	if err != nil {
		t.Fatalf("Map() = %v, want nil", err)
	}
	want := &eventv1.BankAccountChangeRequested{
		BankAccountChangeId:   "bac-100",
		OrganizationId:        "org-100",
		DestinationEvidenceId: "evd-100",
		RequestedByUserId:     "sub-100",
		RequestedAt:           &timestamppb.Timestamp{Seconds: 1_755_432_000, Nanos: 123_456_789},
	}
	if !proto.Equal(mapped.Message, want) {
		t.Errorf("Map().Message = %v, want %v", mapped.Message, want)
	}
	if mapped.Type != bankAccountChangeRequestedType {
		t.Errorf("Map().Type = %q, want %q (PTB-03)", mapped.Type, bankAccountChangeRequestedType)
	}
}

func TestMapperMapsBankAccountChangeApprovedWithTheApprover(t *testing.T) {
	t.Parallel()

	event := domain.BankAccountChangeApproved{
		BankAccountChangeID: "bac-100",
		OrganizationID:      "org-100",
		ApprovedByUserID:    "sub-200",
		At:                  1_755_432_000_123_456_789,
	}

	mapped, err := provider.Mapper{}.Map(event)

	if err != nil {
		t.Fatalf("Map() = %v, want nil", err)
	}
	want := &eventv1.BankAccountChangeApproved{
		BankAccountChangeId: "bac-100",
		OrganizationId:      "org-100",
		ApprovedByUserId:    "sub-200",
		ApprovedAt:          &timestamppb.Timestamp{Seconds: 1_755_432_000, Nanos: 123_456_789},
	}
	if !proto.Equal(mapped.Message, want) {
		t.Errorf("Map().Message = %v, want %v", mapped.Message, want)
	}
	if mapped.Type != bankAccountChangeApprovedType {
		t.Errorf("Map().Type = %q, want %q (PTB-03)", mapped.Type, bankAccountChangeApprovedType)
	}
}

// The bank data is sensitive personal data and never lives in this context (spec,
// BankAccountChange): the events declare exactly the fields of the spec.
func TestTheBankAccountChangeContractsDeclareOnlyTheFieldsOfTheSpec(t *testing.T) {
	t.Parallel()

	for _, c := range []struct {
		message proto.Message
		want    []string
	}{
		{&eventv1.BankAccountChangeRequested{}, []string{"bank_account_change_id", "organization_id", "destination_evidence_id", "requested_by_user_id", "requested_at"}},
		{&eventv1.BankAccountChangeApproved{}, []string{"bank_account_change_id", "organization_id", "approved_by_user_id", "approved_at"}},
	} {
		fields := c.message.ProtoReflect().Descriptor().Fields()
		got := make([]string, 0, fields.Len())
		for i := range fields.Len() {
			got = append(got, string(fields.Get(i).Name()))
		}
		if !slices.Equal(got, c.want) {
			t.Errorf("%s declares %v, want exactly %v", c.message.ProtoReflect().Descriptor().Name(), got, c.want)
		}
	}
}
