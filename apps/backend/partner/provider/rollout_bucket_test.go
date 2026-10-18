package provider_test

import (
	"testing"

	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/ports"
	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/provider"
)

var _ ports.RolloutBucket = provider.RolloutBucket{}

// Each bucket was computed outside this code, by
// int.from_bytes(hashlib.sha256(f"{c}:{t}".encode()).digest()[:8], "big") % 100.
func TestTheRolloutBucketMatchesTheVectorsComputedOutsideTheCode(t *testing.T) {
	for _, tc := range []struct {
		capability domain.CapabilityID
		tenant     port.TenantID
		want       int
	}{
		{"cap-001", "tenant-a", 70},
		{"cap-002", "tenant-b", 76},
		{"cap-003", "tenant-a", 87},
	} {
		first, second := provider.RolloutBucket{}.Bucket(tc.capability, tc.tenant), provider.RolloutBucket{}.Bucket(tc.capability, tc.tenant)

		if first != tc.want || second != tc.want {
			t.Fatalf("Bucket(%s, %s) = %d then %d, want %d both times", tc.capability, tc.tenant, first, second, tc.want)
		}
	}
}
