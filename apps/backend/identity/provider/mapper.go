package provider

import (
	"fmt"
	"time"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	eventv1 "github.com/mateusmacedo/antecipa-market/apps/backend/identity/contract/gen/go/company/identity/event/v1"
	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
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

type Mapper struct{}

func (Mapper) Map(event kernel.DomainEvent) (postgres.Mapped, error) {
	switch e := event.(type) {
	case domain.OrganizationRegistered:
		return mapped(organizationRegisteredType, &eventv1.OrganizationRegistered{
			OrganizationId: string(e.OrganizationID),
			Kinds:          kindsOf(e.Kinds),
			Jurisdiction:   e.Jurisdiction,
			TaxIdType:      taxIDTypeOf(e.TaxIDType),
			TaxIdHash:      e.TaxIDHash,
			TaxIdMasked:    e.TaxIDMasked,
			RegisteredAt:   timestampOf(e.At),
		})
	case domain.EstablishmentAdded:
		return mapped(establishmentAddedType, &eventv1.EstablishmentAdded{
			EstablishmentId: string(e.EstablishmentID),
			OrganizationId:  string(e.OrganizationID),
			TaxIdType:       taxIDTypeOf(e.TaxIDType),
			TaxIdHash:       e.TaxIDHash,
			Receiver:        e.Receiver,
			AddedAt:         timestampOf(e.At),
		})
	case domain.MembershipGranted:
		return mapped(membershipGrantedType, &eventv1.MembershipGranted{
			MembershipId:      string(e.MembershipID),
			OrganizationId:    string(e.OrganizationID),
			UserId:            string(e.UserID),
			Role:              roleOf(e.Role),
			VerificationLevel: verificationLevelOf(e.VerificationLevel),
			GrantedAt:         timestampOf(e.At),
		})
	case domain.MembershipRevoked:
		return mapped(membershipRevokedType, &eventv1.MembershipRevoked{
			MembershipId:   string(e.MembershipID),
			OrganizationId: string(e.OrganizationID),
			UserId:         string(e.UserID),
			Role:           roleOf(e.Role),
			Reason:         e.Reason,
			RevokedAt:      timestampOf(e.At),
		})
	case domain.UserSuspended:
		return mapped(userSuspendedType, &eventv1.UserSuspended{
			UserId:      string(e.UserID),
			Reason:      e.Reason,
			SuspendedAt: timestampOf(e.At),
		})
	case domain.BankAccountChangeRequested:
		return mapped(bankAccountChangeRequestedType, &eventv1.BankAccountChangeRequested{
			BankAccountChangeId:   string(e.BankAccountChangeID),
			OrganizationId:        string(e.OrganizationID),
			DestinationEvidenceId: e.DestinationEvidenceID,
			RequestedByUserId:     e.RequestedByUserID,
			RequestedAt:           timestampOf(e.At),
		})
	case domain.BankAccountChangeApproved:
		return mapped(bankAccountChangeApprovedType, &eventv1.BankAccountChangeApproved{
			BankAccountChangeId: string(e.BankAccountChangeID),
			OrganizationId:      string(e.OrganizationID),
			ApprovedByUserId:    e.ApprovedByUserID,
			ApprovedAt:          timestampOf(e.At),
		})
	default:
		return postgres.Mapped{}, fmt.Errorf("%w: %s", postgres.ErrUnmappedEvent, event.EventName())
	}
}

func mapped(eventType string, message proto.Message) (postgres.Mapped, error) {
	return postgres.Mapped{Message: message, Type: eventType}, nil
}

func timestampOf(at kernel.Instant) *timestamppb.Timestamp {
	return timestamppb.New(time.Unix(0, int64(at)))
}

func kindsOf(kinds domain.OrganizationKinds) []eventv1.OrganizationKind {
	listed := kinds.List()
	out := make([]eventv1.OrganizationKind, 0, len(listed))
	for _, kind := range listed {
		out = append(out, kindOf(kind))
	}
	return out
}

func kindOf(kind domain.OrganizationKind) eventv1.OrganizationKind {
	switch kind {
	case domain.KindClient:
		return eventv1.OrganizationKind_ORGANIZATION_KIND_CLIENT
	case domain.KindFunder:
		return eventv1.OrganizationKind_ORGANIZATION_KIND_FUNDER
	case domain.KindPartner:
		return eventv1.OrganizationKind_ORGANIZATION_KIND_PARTNER
	case domain.KindPlatform:
		return eventv1.OrganizationKind_ORGANIZATION_KIND_PLATFORM
	default:
		return eventv1.OrganizationKind_ORGANIZATION_KIND_UNSPECIFIED
	}
}

func taxIDTypeOf(kind domain.TaxIDType) eventv1.TaxIdType {
	switch kind {
	case domain.TaxIDTypeCNPJ:
		return eventv1.TaxIdType_TAX_ID_TYPE_CNPJ
	case domain.TaxIDTypeCPF:
		return eventv1.TaxIdType_TAX_ID_TYPE_CPF
	default:
		return eventv1.TaxIdType_TAX_ID_TYPE_UNSPECIFIED
	}
}

func roleOf(role domain.Role) eventv1.MembershipRole {
	switch role {
	case domain.RoleOperator:
		return eventv1.MembershipRole_MEMBERSHIP_ROLE_OPERATOR
	case domain.RoleAuditor:
		return eventv1.MembershipRole_MEMBERSHIP_ROLE_AUDITOR
	default:
		return eventv1.MembershipRole_MEMBERSHIP_ROLE_UNSPECIFIED
	}
}

func verificationLevelOf(level domain.VerificationLevel) eventv1.VerificationLevel {
	switch level {
	case domain.VerificationChannelOwnership:
		return eventv1.VerificationLevel_VERIFICATION_LEVEL_CHANNEL_OWNERSHIP
	case domain.VerificationPersonalDataChecked:
		return eventv1.VerificationLevel_VERIFICATION_LEVEL_PERSONAL_DATA_CHECKED
	default:
		return eventv1.VerificationLevel_VERIFICATION_LEVEL_UNSPECIFIED
	}
}
