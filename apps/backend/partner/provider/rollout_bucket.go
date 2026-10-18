package provider

import (
	"crypto/sha256"
	"encoding/binary"

	port "github.com/mateusmacedo/dmpf/libs/backend/go/ports"

	"github.com/mateusmacedo/antecipa-market/apps/backend/partner/domain"
)

const rolloutBuckets = 100

type RolloutBucket struct{}

func (RolloutBucket) Bucket(capabilityID domain.CapabilityID, tenant port.TenantID) int {
	sum := sha256.Sum256([]byte(string(capabilityID) + ":" + string(tenant)))
	return int(binary.BigEndian.Uint64(sum[:8]) % rolloutBuckets)
}
