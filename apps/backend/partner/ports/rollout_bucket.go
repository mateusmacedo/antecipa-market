package ports

import (
	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
)

// RolloutBucket places a tenant in one of the 100 buckets of a capability. It
// is a port because the depguard of .golangci.yml denies crypto/* to the
// domain and application blocks.
type RolloutBucket interface {
	Bucket(capabilityID domain.CapabilityID, tenant port.TenantID) int
}
