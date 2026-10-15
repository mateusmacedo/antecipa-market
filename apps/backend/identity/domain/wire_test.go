package domain_test

import (
	"testing"

	kernel "github.com/mateusmacedo/dmpf/libs/backend/go/domain"

	"github.com/mateusmacedo/antecipa-market/apps/backend/identity/domain"
)

// The event name is published with the event: a rename in Go must not reach it.
func TestEventNamesKeepTheirPublishedValues(t *testing.T) {
	events := map[string]kernel.DomainEvent{
		"identity-organization.organization.registered": domain.OrganizationRegistered{},
		"identity-organization.establishment.added":     domain.EstablishmentAdded{},
		"identity-organization.membership.granted":      domain.MembershipGranted{},
		"identity-organization.membership.revoked":      domain.MembershipRevoked{},
		"identity-organization.user-account.suspended":  domain.UserSuspended{},

		"identity-organization.bank-account-change.requested": domain.BankAccountChangeRequested{},
		"identity-organization.bank-account-change.approved":  domain.BankAccountChangeApproved{},
	}
	for want, event := range events {
		if got := event.EventName(); got != want {
			t.Errorf("EventName() = %q, want the published value %q", got, want)
		}
	}
}
