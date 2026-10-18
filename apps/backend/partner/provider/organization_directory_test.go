package provider_test

import (
	"context"
	"errors"
	"testing"

	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/ports"
	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/provider"
)

var _ ports.OrganizationDirectory = provider.UnconfiguredOrganizationDirectory{}

func TestTheUnconfiguredDirectoryFailsClosedForEveryOrganization(t *testing.T) {
	for _, id := range []domain.OrganizationID{"org-1", "org-none", ""} {
		kinds, err := provider.UnconfiguredOrganizationDirectory{}.FindOrganization(context.Background(), "acme", id)

		if !errors.Is(err, provider.ErrOrganizationDirectoryUnconfigured) || kinds != nil {
			t.Fatalf("FindOrganization(%q) = %v, %v; want the technical refusal and no kind", id, kinds, err)
		}
		if errors.Is(err, port.ErrNotFound) {
			t.Fatalf("FindOrganization(%q) = %v, want a technical failure rather than an organization not found", id, err)
		}
	}
}
